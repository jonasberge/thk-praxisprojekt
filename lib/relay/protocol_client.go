package relay

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"google.golang.org/protobuf/proto"
	"io"
	"log"
	"net"
	. "projekt/core/lib/device"
	"projekt/core/lib/packet"
	"projekt/core/lib/protocol"
)

var (
	ErrAlreadyConnected       = errors.New("this identity is already connected to the relay")
	ErrAlreadyRequested       = errors.New("a request for this identity is already in progress")
	ErrTargetUnavailable      = errors.New("the target is not available on this server")
	ErrSessionExists          = errors.New("the requested session already exists")
	ErrUnknownSession         = errors.New("the session does not exists")
	ErrSessionAlreadyAccepted = errors.New("the session was already accepted")
	ErrInitiatorUnavailable   = errors.New("the initiator of the session is not available anymore")
	ErrClosed                 = net.ErrClosed
)

// ClientProtocol is the protocol implementation that is used by a relay client.
type ClientProtocol struct {
	protocol Protocol
	log      *log.Logger
	conn     net.Conn

	newRequest      chan requestInput
	requestResponse chan requestResponse
	deleteRequest   chan Identity
	newInvitation   chan *SessionInvitation
	newAccept       chan *SessionAccepted
	waitAccept      chan waitAccept
	newResult       chan establishedOrFailed
	waitResult      chan waitResult
	done            chan struct{}
}

type requestInput struct {
	targetIdentity Identity
	additionalData []byte
	out            chan requestOutput
}

type requestResponse struct {
	targetIdentity Identity
	requestOutput  requestOutput
}

type requestOutput struct {
	sessionDescription *SessionDescription
	err                error
}

type waitAccept struct {
	targetIdentity Identity
	out            chan *SessionAccepted
}

type establishedOrFailed struct {
	established *SessionEstablished
	failed      *SessionFailed
}

type waitResult struct {
	initiatorIdentity Identity
	out               chan establishedOrFailed
}

// NewClientProtocol creates a new instance of a ClientProtocol.
// It takes a net.Conn that is used to talk to a relay server.
// The server must speak the ServerProtocol.
func NewClientProtocol(serverConn net.Conn) *ClientProtocol {
	return &ClientProtocol{
		protocol:        Protocol{},
		log:             log.New(log.Writer(), "relay.client: ", log.Flags()),
		conn:            serverConn,
		newRequest:      make(chan requestInput),
		requestResponse: make(chan requestResponse),
		deleteRequest:   make(chan Identity),
		newInvitation:   make(chan *SessionInvitation),
		newAccept:       make(chan *SessionAccepted),
		waitAccept:      make(chan waitAccept),
		newResult:       make(chan establishedOrFailed),
		waitResult:      make(chan waitResult),
		done:            make(chan struct{}),
	}
}

func (c *ClientProtocol) Start(ctx context.Context) (err error) {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			// Close the connection to make the below call to Read fail.
			if e := c.conn.Close(); e != nil {
				c.log.Println("start: failed to close connection:", e)
			}
		case <-done:
			return
		}
	}()

	// TODO Maybe put this into the messageHandler.
	data, err := packet.DecodeFrom(c.conn)
	close(done)
	if err != nil {
		c.handleConnErr(err)
		return
	}
	select {
	case <-ctx.Done():
		// The goroutine above might have accidentally closed the connection
		// if the context was done after reading but before closing the done channel.
		// This makes sure an error is returned in case that has happened.
		return ctx.Err()
	default:
	}

	message, err := c.protocol.UnwrapMessage(data)
	if err != nil {
		return
	}
	switch message.(type) {
	case *Connected:
	case *AlreadyConnected:
		return ErrAlreadyConnected
	default:
		return protocol.UnexpectedMessage
	}
	go c.messageHandler()
	go c.requestHandler()
	go c.sessionHandler()
	return
}

func (c *ClientProtocol) messageHandler() {
	// FIXME This is copied from ServerProtocol.handleClient and only slightly changed.
	defer func() {
		if err := c.Close(); err != nil {
			c.log.Println("message handler: failed to close client:", err)
		}
	}()
	for {
		data, err := packet.DecodeFrom(c.conn)
		if err != nil {
			c.log.Println("message handler: failed to decode packet:", err)
			return
		}
		select {
		case <-c.done:
			return
		default:
		}
		message, err := c.protocol.UnwrapMessage(data)
		if err != nil {
			c.log.Println("message handler: failed to unwrap message:", err)
			return
		}
		err = c.handleMessage(message)
		if err != nil {
			c.log.Println("message handler: failed to handle message:", err)
			return
		}
	}
}

func (c *ClientProtocol) handleMessage(message proto.Message) (err error) {
	//c.log.Println("Handling message:", message.ProtoReflect().Type().Descriptor().Name())
	switch m := message.(type) {
	case *SessionFailed:
		c.requestResponse <- requestResponse{
			targetIdentity: m.PeerIdentity,
			requestOutput: requestOutput{
				err: reasonToError(m.Reason),
			},
		}
		c.newResult <- establishedOrFailed{
			failed: m,
		}
	case *SessionCreated:
		c.requestResponse <- requestResponse{
			targetIdentity: m.TargetIdentity,
			requestOutput: requestOutput{
				sessionDescription: m.Session,
			},
		}
	case *SessionAccepted:
		c.newAccept <- m
	case *SessionInvitation:
		select {
		// FIXME Buffer the invitations.
		//  Otherwise some might get lost in a brief moment where there is no receiver.
		case c.newInvitation <- m:
		default:
			c.log.Println("handle message: discarded invitation from:", hex.EncodeToString(m.InitiatorIdentity))
		}
	case *SessionEstablished:
		c.newResult <- establishedOrFailed{
			established: m,
		}
	default:
		return fmt.Errorf("%w: %v", protocol.UnexpectedMessage, m.ProtoReflect().Descriptor().Name())
	}
	return
}

func (c *ClientProtocol) requestHandler() {
	requests := make(map[identityHex]chan requestOutput, 4)
	for {
		select {
		case request := <-c.newRequest:
			id := hex.EncodeToString(request.targetIdentity)
			if _, has := requests[id]; has {
				offerRequestOutput(request.out, requestOutput{
					err: ErrAlreadyRequested,
				})
			} else {
				requests[id] = request.out
				go c.writeRequest(request.targetIdentity, request.additionalData, request.out)
			}
		case response := <-c.requestResponse:
			id := hex.EncodeToString(response.targetIdentity)
			if out, ok := requests[id]; ok {
				offerRequestOutput(out, response.requestOutput)
			}
		case targetIdentity := <-c.deleteRequest:
			id := hex.EncodeToString(targetIdentity)
			if _, has := requests[id]; !has {
				panic("attempting to close a request which does not exist")
			}
			delete(requests, id)
		case <-c.done:
			return
		}
	}
}

func (c *ClientProtocol) sessionHandler() {
	accepts := make(map[identityHex]chan *SessionAccepted, 4)
	results := make(map[identityHex]chan establishedOrFailed, 4)
	for {
		select {
		case request := <-c.waitAccept:
			id := hex.EncodeToString(request.targetIdentity)
			if _, has := accepts[id]; has {
				// TODO Handle this error differently.
				panic("already waiting for an accept for this identity")
			}
			accepts[id] = request.out
		case request := <-c.waitResult:
			id := hex.EncodeToString(request.initiatorIdentity)
			if _, has := results[id]; has {
				// TODO Handle this error differently.
				panic("already waiting for a result for this identity")
			}
			results[id] = request.out
		case accept := <-c.newAccept:
			id := hex.EncodeToString(accept.TargetIdentity)
			out, ok := accepts[id]
			if !ok {
				c.log.Println("received an accept message which is not being waited for")
				continue
			}
			out <- accept
			delete(accepts, id)
		case result := <-c.newResult:
			var id string
			if result.failed != nil {
				id = hex.EncodeToString(result.failed.PeerIdentity)
			} else if result.established != nil {
				id = hex.EncodeToString(result.established.InitiatorIdentity)
			} else {
				panic("result must be either failed or established")
			}
			out, ok := results[id]
			if !ok {
				c.log.Println("received a result which is not being waited for")
				continue
			}
			out <- result
			delete(results, id)
		}
	}
}

func (c *ClientProtocol) writeRequest(targetIdentity Identity, additionalData []byte, out chan requestOutput) {
	data, err := c.protocol.WrapMessage(&RequestSession{
		TargetIdentity: targetIdentity,
		AdditionalData: additionalData,
	})
	if err != nil {
		offerRequestOutput(out, requestOutput{
			err: fmt.Errorf("failed to wrap request message: %w", err),
		})
		return
	}
	_, err = packet.New(data).WriteTo(c.conn)
	if err != nil {
		offerRequestOutput(out, requestOutput{
			err: fmt.Errorf("failed to write request message: %w", err),
		})
		c.handleConnErr(err)
		return
	}
}

// Request performs a request for creation of a session with a specific target device.
// The device is identified by its target device.Identity.
// Additional data can be transmitted in the form a byte slice.
func (c *ClientProtocol) Request(ctx context.Context, targetIdentity Identity, additionalData []byte) (info SessionInfo, err error) {
	out := make(chan requestOutput)
	c.newRequest <- requestInput{targetIdentity, additionalData, out}
	select {
	case output := <-out:
		if output.err == nil {
			info = SessionInfo{
				Description:  output.sessionDescription,
				PeerIdentity: targetIdentity,
			}
		}
		err = output.err
	case <-ctx.Done():
		err = ctx.Err()
	}
	c.deleteRequest <- targetIdentity
	return
}

// ReadAccept blocks until the target device of a Request has accepted that request.
// The request is identified by the target device.Identity that has been passed to Request.
func (c *ClientProtocol) ReadAccept(ctx context.Context, targetIdentity Identity) (additionalData []byte, err error) {
	out := make(chan *SessionAccepted)
	c.waitAccept <- waitAccept{targetIdentity, out}
	select {
	case accepted := <-out:
		additionalData = accepted.AdditionalData
	case <-c.done:
		err = ErrClosed
	}
	return
}

// ReadInvitation blocks until an invitation for a session has been received from the server,
// an error occurs or the protocol or underlying connection is closed.
// Next to the SessionInfo additional data is read that might have been added to the request.
func (c *ClientProtocol) ReadInvitation() (info SessionInfo, additionalData []byte, err error) {
	select {
	case invitation := <-c.newInvitation:
		info = SessionInfo{
			Description:  invitation.Session,
			PeerIdentity: invitation.InitiatorIdentity,
		}
		additionalData = invitation.AdditionalData
	case <-c.done:
		err = ErrClosed
	}
	return
}

// Accept accepts an invitation that has been read with ReadInvitation.
// The invitation is identified by the device.Identity that was in the returned SessionInfo.
// Additional data can be associated that should be communicated to the initiator of the request.
// This method blocks until the server acknowledges that the session is successfully established.
func (c *ClientProtocol) Accept(initiatorIdentity Identity, additionalData []byte) (err error) {
	out := make(chan establishedOrFailed)
	c.waitResult <- waitResult{initiatorIdentity, out}
	data, err := c.protocol.WrapMessage(&AcceptSession{
		InitiatorIdentity: initiatorIdentity,
		AdditionalData:    additionalData,
	})
	if err != nil {
		return
	}
	_, err = packet.New(data).WriteTo(c.conn)
	if err != nil {
		return
	}
	select {
	case result := <-out:
		if result.failed != nil {
			err = reasonToError(result.failed.Reason)
		}
	case <-c.done:
		err = ErrClosed
	}
	return
}

func (c *ClientProtocol) Close() error {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
	c.log.SetOutput(io.Discard)
	return c.conn.Close()
}

func (c *ClientProtocol) handleConnErr(err error) {
	if errors.Is(err, net.ErrClosed) {
		// If the connection is closed then the protocol is done.
		select {
		case <-c.done:
		default:
			close(c.done)
		}
		c.log.SetOutput(io.Discard)
	}
}

func (c *ClientProtocol) write(conn net.Conn, message proto.Message) (ok bool) {
	// TODO This could also be refactored into the protocol type.
	var err error
	defer func() {
		// TODO Use the ok value.
		if err != nil {
			e := conn.Close()
			if e != nil {
				c.log.Println("failed to close connection:", e)
			}
		}
	}()
	data, err := c.protocol.WrapMessage(message)
	if err != nil {
		c.log.Println("failed to wrap message:", err)
		return
	}
	_, err = packet.New(data).WriteTo(conn)
	if err != nil {
		c.log.Println("failed to write message:", err)
		return
	}
	return true
}

// offerRequestOutput attempts to send an output to an output channel.
// It either sends the data or it returns if there is no receiver.
func offerRequestOutput(out chan requestOutput, output requestOutput) (ok bool) {
	select {
	case out <- output:
		return true
	default:
		return false
	}
}

func reasonToError(reason Error) error {
	switch reason {
	case ErrorTargetUnavailable:
		return ErrTargetUnavailable
	case ErrorSessionExists:
		return ErrSessionExists
	case ErrorUnknownSession:
		return ErrUnknownSession
	case ErrorSessionAlreadyAccepted:
		return ErrSessionAlreadyAccepted
	case ErrorInitiatorUnavailable:
		return ErrInitiatorUnavailable
	default:
		return errors.New("received an invalid error response from server")
	}
}
