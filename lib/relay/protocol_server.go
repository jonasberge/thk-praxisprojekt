package relay

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
	"io"
	"log"
	"net"
	. "projekt/core/lib/device"
	"projekt/core/lib/packet"
	"projekt/core/lib/protocol"
	"time"
)

// SessionKeySize is the byte size of a session key.
const SessionKeySize = 16

// TODO: Think of something more efficient than encoding
//  byte arrays to hex every single time for usage as map keys.

func newSessionKey() []byte {
	// TODO: Make a pool of keys for faster retrieval of a new key.
	key := make([]byte, SessionKeySize)
	_, err := rand.Read(key)
	if err != nil {
		log.Fatalln("failed to create new session key:", err)
	}
	return key
}

type sessionIdentifier string

// newSessionIdentifier creates a new unique identifier for a session between two identities.
// The order of the arguments is important. A different order will give a different identifier.
func newSessionIdentifier(id1 Identity, id2 Identity) sessionIdentifier {
	a := hex.EncodeToString(id1)
	b := hex.EncodeToString(id2)
	// If the order were not encoded in this identifier
	// then an AcceptSession message could be sent by the initiator or the target
	// and the session identifier would look the same.
	return sessionIdentifier(a + b)
}

// ServerProtocol is the protocol implementation that is used by a relay server.
type ServerProtocol struct {
	protocol Protocol
	log      *log.Logger

	newClient      chan identityConn
	getClient      chan identityOutConn
	deleteClient   chan Identity
	newSession     chan sessionInput
	acceptSession  chan sessionInput
	deleteSession  chan sessionIdentifier
	getSessionChan chan sessionKeyOutConn
	done           chan struct{}

	sessionPort int
}

type identityConn struct {
	identity Identity
	conn     net.Conn
}

type identityOutConn struct {
	identity Identity
	out      chan net.Conn
}

type sessionInput struct {
	initiatorIdentity Identity
	targetIdentity    Identity
	out               chan sessionOutput
}

type sessionOutput struct {
	initiatorDescription *SessionDescription
	targetDescription    *SessionDescription
	error                Error
}

type sessionHandle struct {
	initiatorIdentity Identity
	targetIdentity    Identity
	keys              [2][]byte
	accepted          chan struct{}
}

type sessionKeyHandle struct {
	id     sessionIdentifier
	inConn chan net.Conn
}

type sessionKeyOutConn struct {
	key []byte
	out chan chan net.Conn
}

// NewServerProtocol creates a new instance of a ServerProtocol.
// The sessionPort is the port that clients should connect to in order to join a session.
// It is communicated to clients within a SessionDescription.
func NewServerProtocol(sessionPort int) *ServerProtocol {
	return &ServerProtocol{
		protocol:       Protocol{},
		log:            log.New(log.Writer(), "relay.server: ", log.Flags()),
		newClient:      make(chan identityConn),
		getClient:      make(chan identityOutConn),
		deleteClient:   make(chan Identity),
		newSession:     make(chan sessionInput),
		acceptSession:  make(chan sessionInput),
		deleteSession:  make(chan sessionIdentifier),
		getSessionChan: make(chan sessionKeyOutConn),
		done:           make(chan struct{}),
		sessionPort:    sessionPort,
	}
}

func (s *ServerProtocol) Start() error {
	go s.clientHandler()
	go s.sessionHandler()
	return nil
}

func (s *ServerProtocol) clientHandler() {
	clients := make(map[identityHex]identityConn, 128)
	for {
		select {
		case client := <-s.newClient:
			id := hex.EncodeToString(client.identity)
			if _, has := clients[id]; has {
				go func() {
					s.write(client.conn, &AlreadyConnected{})
					client.conn.Close()
				}()
				continue
			}
			clients[id] = client
			go func() {
				s.log.Println("new client:", id)
				if s.write(client.conn, &Connected{}) {
					go s.handleClient(client.identity, client.conn)
				}
			}()
		case request := <-s.getClient:
			id := hex.EncodeToString(request.identity)
			client, ok := clients[id]
			if ok {
				request.out <- client.conn
			} else {
				request.out <- nil
			}
		case identity := <-s.deleteClient:
			id := hex.EncodeToString(identity)
			if _, has := clients[id]; !has {
				panic("attempting to delete a client that does not exist")
			}
			delete(clients, id)
		case <-s.done:
			for _, client := range clients {
				go client.conn.Close()
			}
			return
		}
	}
}

func (s *ServerProtocol) sessionHandler() {
	sessions := make(map[sessionIdentifier]*sessionHandle, 128)
	keys := make(map[string]sessionKeyHandle, 256)
	for {
		select {
		case session := <-s.newSession:
			id := newSessionIdentifier(session.initiatorIdentity, session.targetIdentity)
			if _, has := sessions[id]; has {
				session.out <- sessionOutput{
					error: ErrorSessionExists,
				}
				continue
			}
			key1, key2 := newSessionKey(), newSessionKey()
			ch1, ch2 := make(chan net.Conn), make(chan net.Conn)
			keys[hex.EncodeToString(key1)] = sessionKeyHandle{id, ch1}
			keys[hex.EncodeToString(key2)] = sessionKeyHandle{id, ch2}
			accepted := make(chan struct{})
			sessions[id] = &sessionHandle{
				initiatorIdentity: session.initiatorIdentity,
				targetIdentity:    session.targetIdentity,
				keys:              [2][]byte{key1, key2},
				accepted:          accepted,
			}
			go s.handleSession(id, ch1, ch2, accepted)
			session.out <- sessionOutput{
				initiatorDescription: s.createSessionDescription(key1),
				targetDescription:    s.createSessionDescription(key2),
				error:                ErrorNone,
			}
		case session := <-s.acceptSession:
			id := newSessionIdentifier(session.initiatorIdentity, session.targetIdentity)
			handle, ok := sessions[id]
			if !ok {
				session.out <- sessionOutput{
					error: ErrorUnknownSession,
				}
				continue
			}
			select {
			case <-handle.accepted:
				session.out <- sessionOutput{
					error: ErrorSessionAlreadyAccepted,
				}
				continue
			default:
				close(handle.accepted)
				session.out <- sessionOutput{
					error: ErrorNone,
				}
			}
		case id := <-s.deleteSession:
			handle, ok := sessions[id]
			if !ok {
				panic("attempting to delete a session that does not exist")
			}
			for _, key := range handle.keys {
				delete(keys, hex.EncodeToString(key))
			}
			delete(sessions, id)
		case session := <-s.getSessionChan:
			key := hex.EncodeToString(session.key)
			handle, ok := keys[key]
			if ok {
				session.out <- handle.inConn
			} else {
				session.out <- nil
			}
		case <-s.done:
			return
		}
	}
}

func (s *ServerProtocol) handleClient(identity Identity, conn net.Conn) {
	defer func() {
		s.deleteClient <- identity
		if err := conn.Close(); err != nil {
			s.log.Println("handle client: failed to close connection:", err)
		}
	}()
	for {
		data, err := packet.DecodeFrom(conn)
		if err != nil {
			if err != io.EOF {
				s.log.Println("handle client: failed to decode packet:", err)
			}
			return
		}
		select {
		case <-s.done:
			return
		default:
		}
		message, err := s.protocol.UnwrapMessage(data)
		if err != nil {
			s.log.Println("handle client: failed to unwrap message:", err)
			return
		}
		s.log.Println("<-", message.ProtoReflect().Descriptor().Name())
		err = s.handleMessage(conn, identity, message)
		if err != nil {
			s.log.Println("handle client: failed to handle message:", err)
			return
		}
	}
}

func (s *ServerProtocol) handleMessage(conn net.Conn, identity Identity, message proto.Message) error {
	switch m := message.(type) {
	case *RequestSession:
		s.handleSessionRequest(conn, identity, m)
	case *AcceptSession:
		s.handleSessionAccept(conn, identity, m)
	default:
		// TODO handle an unexpected message differently
		return protocol.UnexpectedMessage
	}
	return nil
}

func (s *ServerProtocol) handleSessionRequest(conn net.Conn, identity Identity, request *RequestSession) {
	clientOut := make(chan net.Conn)
	s.getClient <- identityOutConn{request.TargetIdentity, clientOut}
	targetConn := <-clientOut
	if targetConn == nil {
		go s.write(conn, &SessionFailed{
			Reason:       ErrorTargetUnavailable,
			PeerIdentity: request.TargetIdentity,
		})
		return
	}
	sessionOut := make(chan sessionOutput)
	s.newSession <- sessionInput{identity, request.TargetIdentity, sessionOut}
	session := <-sessionOut
	if session.error != ErrorNone {
		go s.write(conn, &SessionFailed{
			Reason:       session.error,
			PeerIdentity: request.TargetIdentity,
		})
		return
	}
	go s.write(conn, &SessionCreated{
		Session:        session.initiatorDescription,
		TargetIdentity: request.TargetIdentity,
	})
	go s.write(targetConn, &SessionInvitation{
		Session:           session.targetDescription,
		InitiatorIdentity: identity,
		AdditionalData:    request.AdditionalData,
	})
}

func (s *ServerProtocol) handleSessionAccept(conn net.Conn, identity Identity, accept *AcceptSession) {
	sessionOut := make(chan sessionOutput)
	s.acceptSession <- sessionInput{accept.InitiatorIdentity, identity, sessionOut}
	session := <-sessionOut
	if session.error != ErrorNone {
		go s.write(conn, &SessionFailed{
			Reason:       session.error,
			PeerIdentity: accept.InitiatorIdentity,
		})
		return
	}
	clientOut := make(chan net.Conn)
	s.getClient <- identityOutConn{accept.InitiatorIdentity, clientOut}
	initiatorConn := <-clientOut
	if initiatorConn == nil {
		go s.write(conn, &SessionFailed{
			Reason:       ErrorInitiatorUnavailable,
			PeerIdentity: accept.InitiatorIdentity,
		})
		return
	}
	go s.write(initiatorConn, &SessionAccepted{
		TargetIdentity: identity,
		AdditionalData: accept.AdditionalData,
	})
	go s.write(conn, &SessionEstablished{
		InitiatorIdentity: accept.InitiatorIdentity,
	})
}

const SessionTimeout = 30 * time.Second
const SessionBufSize = 1024

// TODO Till how far can the buffer size be decreased and still be efficient?
// TODO Limit the number of connections or the available RAM to the server.
//  Otherwise sessions could fill it up indefinitely.

func (s *ServerProtocol) handleSession(id sessionIdentifier, ch1, ch2 chan net.Conn, accepted chan struct{}) {
	ctx, cancel := context.WithTimeout(context.Background(), SessionTimeout)
	defer cancel()
	var conn1, conn2 net.Conn
	var eg errgroup.Group
	eg.Go(func() error {
		select {
		case conn1 = <-ch1:
			return writeSessionResponse(conn1, SessionStateConnected)
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	eg.Go(func() error {
		select {
		case conn2 = <-ch2:
			return writeSessionResponse(conn2, SessionStateConnected)
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	err := eg.Wait() // Both clients connected or an error occurred.
	if err != nil {
		s.log.Println("handle session: failed to accept client:", err)
		if conn1 != nil {
			_ = conn1.Close()
		}
		if conn2 != nil {
			_ = conn2.Close()
		}
		return
	}
	<-accepted // The target client accepted the session

	// TODO Better failure handling.
	//  On disconnect of a client we could read from the respective channel that was passed as a parameter.
	//  The client can reconnect with the same key and then a write attempt to that channel
	//  in the AddSessionClient function will succeed again since we are receiving on it.
	//  ...

	relay := func(src, dst net.Conn, done chan struct{}) {
		buf := make([]byte, SessionBufSize)
		for {
			n, e := src.Read(buf)
			if e != nil {
				close(done)
				return
			}
			select {
			case <-s.done:
				return
			default:
			}
			data := buf[:n]
			_, err = dst.Write(data)
			if err != nil {
				close(done)
				return
			}
			select {
			case <-s.done:
				return
			default:
			}
		}
	}

	d1 := make(chan struct{})
	d2 := make(chan struct{})
	go relay(conn1, conn2, d1)
	go relay(conn2, conn1, d2)

	select {
	case <-d1:
		s.deleteSession <- id
	case <-d2:
		s.deleteSession <- id
	case <-s.done:
	}
	go conn1.Close()
	go conn2.Close()
}

// AddClient adds a client with a specific device.Identity to the server protocol.
// The server communicates with that client over the passed net.Conn
// The client must speak the ClientProtocol.
func (s *ServerProtocol) AddClient(identity Identity, conn net.Conn) {
	s.newClient <- identityConn{identity, conn}
}

// AddSessionClient adds a client for the session protocol of the relay server.
// The server communicates with that client over the passed net.Conn.
// The client must speak the SessionClientProtocol.
func (s *ServerProtocol) AddSessionClient(conn net.Conn) {
	go func() {
		data, err := packet.DecodeFrom(conn)
		if err != nil {
			s.log.Println("session client: failed to read packet:", err)
			return
		}
		message := &AuthenticateSession{}
		err = proto.Unmarshal(data, message)
		if err != nil {
			s.log.Println("session client: failed to unmarshal authentication message:", err)
			return
		}
		out := make(chan chan net.Conn)
		s.getSessionChan <- sessionKeyOutConn{message.Key, out}
		in := <-out
		if in == nil {
			// Nobody is waiting for this key so it is invalid.
			_ = writeSessionResponse(conn, SessionStateInvalidKey)
			return
		}
		select {
		case in <- conn:
		default:
			// No receiver, so the client for this key is already connected.
			_ = writeSessionResponse(conn, SessionStateAlreadyConnected)
		}
	}()
}

func (s *ServerProtocol) Close() error {
	close(s.done)
	s.log.SetOutput(io.Discard)
	return nil
}

func (s *ServerProtocol) write(conn net.Conn, message proto.Message) (ok bool) {
	// TODO This can also be refactored into the protocol type.
	//  A counterpart for reading would also need to be supplied of course.
	var err error
	defer func() {
		// TODO Use the ok value.
		if err != nil {
			e := conn.Close()
			if e != nil {
				s.log.Println("failed to close connection:", e)
			}
		}
	}()
	data, err := s.protocol.WrapMessage(message)
	if err != nil {
		s.log.Println("failed to wrap message:", err)
		return
	}
	s.log.Println("->", message.ProtoReflect().Descriptor().Name())
	_, err = packet.New(data).WriteTo(conn)
	if err != nil {
		s.log.Println("failed to write message:", err)
		return
	}
	return true
}

func (s *ServerProtocol) writeAndClose(conn net.Conn, message proto.Message) {
	if s.write(conn, message) {
		err := conn.Close()
		if err != nil {
			s.log.Println("failed to close connection:", err)
		}
	}
}

func (s *ServerProtocol) createSessionDescription(key []byte) *SessionDescription {
	return &SessionDescription{
		Key:  key,
		Port: uint32(s.sessionPort),
	}
}

func writeSessionResponse(conn net.Conn, state SessionState) (err error) {
	data, err := proto.Marshal(&SessionResponse{State: state})
	if err != nil {
		return
	}
	_, err = packet.New(data).WriteTo(conn)
	return
}
