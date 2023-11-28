package local

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	. "github.com/flynn/noise"
	"google.golang.org/protobuf/proto"
	"log"
	"net"
	"projekt/core/lib/beacon"
	. "projekt/core/lib/device"
	"projekt/core/lib/local/message"
	"projekt/core/lib/network"
	"projekt/core/lib/packet"
	"projekt/core/lib/secure"
	"strings"
	"sync/atomic"
	"time"
)

const (
	IoTimeout     = 10 * time.Second
	RequestMaxAge = 20 * time.Second
)

var (
	ConnectionRefused = errors.New("the peer refused the connection")
)

type Client interface {
	Listen() error
	Close() error

	Connect(ctx context.Context, targetIdentity Identity, targetKey HMACKey, addrs ...string) (conn secure.Conn, err error)
	Accept() (conn secure.Conn, err error)

	Identity() Identity
	HmacKey() HMACKey
	CipherSuite() CipherSuite
}

type Dialer interface {
	DialContext(ctx context.Context, network string, address string) (net.Conn, error)
}

type client struct {
	port        int
	keyPair     KeyPair
	deviceKey   HMACKey
	trustStore  TrustStore
	cipherSuite CipherSuite

	requestId uint64

	tracker  network.Tracker
	beacon   beacon.Beacon
	listener net.Listener
	dialer   Dialer
	isDirty  int32

	requestHandlers  chan interface{}
	responseHandlers chan interface{}

	broadcasts chan broadcastPacket
	unicasts   chan unicastPacket

	requests  chan requestConn
	responses chan responseConn

	done chan struct{}
}

type requestConn struct {
	request Request
	conn    net.Conn
}

type responseConn struct {
	response Response
	conn     net.Conn
}

type broadcastPacket struct {
	ctx    context.Context
	packet packet.Interface
}

type unicastPacket struct {
	ctx    context.Context
	packet packet.Interface
	addr   string
}

type registerResponseHandler struct {
	requestId RequestId
	handler   ResponseHandler
}

type removeResponseHandler struct {
	requestId RequestId
	handler   ResponseHandler
}

type TrustStore interface {
	IsTrusted(identity Identity) bool
	IsX25519Trusted(identity X25519Identity) bool
}

func NewClient(port int, keyPair KeyPair, key HMACKey,
	trustStore TrustStore, cipherSuite CipherSuite) Client {
	return &client{
		port:             port,
		keyPair:          keyPair,
		deviceKey:        key,
		trustStore:       trustStore,
		cipherSuite:      cipherSuite,
		requestId:        0,
		tracker:          nil,
		beacon:           nil,
		listener:         nil,
		requestHandlers:  make(chan interface{}),
		responseHandlers: make(chan interface{}),
		broadcasts:       make(chan broadcastPacket),
		unicasts:         make(chan unicastPacket),
		requests:         make(chan requestConn),
		responses:        make(chan responseConn),
		done:             make(chan struct{}),
	}
}

func (c *client) Identity() Identity {
	return c.keyPair.Public
}

func (c *client) HmacKey() HMACKey {
	return c.deviceKey
}

func (c *client) CipherSuite() CipherSuite {
	return c.cipherSuite
}

// testListen starts listeners and connection handlers just like Listen
// but takes a beacon.Beacon and a net.Listener as an argument for simpler testability
// as these are the only components that are used for communication on the network.
// Both arguments are expected to be usable without calling any additional initialization functions.
func (c *client) testListen(tracker network.Tracker, beacon beacon.Beacon, listener net.Listener, dialer Dialer) {
	c.tracker = tracker
	c.beacon = beacon
	c.listener = listener
	c.dialer = dialer
	go c.requestBroker()
	go c.responseBroker()
	go c.outgoingBroadcastHandler()
	go c.incomingUnicastHandler()
	return
}

// Listen starts listeners and connection handlers for communicating with other clients.
// This method must be called first after the Client has been instantiated with NewClient.
// This method may only be called once.
func (c *client) Listen() (err error) {
	if !atomic.CompareAndSwapInt32(&c.isDirty, 0, 1) {
		panic("Listen was invoked more than once")
	}
	c.beacon = beacon.NewBroadcast(context.Background(), c.port) // beacon.NewBeacon()
	err = c.beacon.Listen()
	if err != nil {
		return
	}
	c.listener, err = net.ListenTCP("tcp", &net.TCPAddr{Port: c.port})
	if err != nil {
		c.beacon.Close()
		return
	}
	c.tracker = network.NewTracker()
	c.dialer = &net.Dialer{
		Timeout: IoTimeout,
	}
	go c.requestBroker()
	go c.responseBroker()
	go c.outgoingBroadcastHandler()
	go c.incomingUnicastHandler()
	return
}

func (c *client) Close() (err error) {
	close(c.done)
	e1 := c.beacon.Close()
	e2 := c.listener.Close()
	if e1 != nil {
		err = e1
	} else if e2 != nil {
		err = e2
	}
	return
}

func (c *client) requestBroker() {
	for {
		select {
		// ...
		case <-c.done:
			return
		}
	}
}

// responseBroker routes incoming responses and their respective network connection
// to the ResponseHandler that was registered for the request's RequestId.
func (c *client) responseBroker() {
	handlers := make(map[RequestId]ResponseHandler, 4)
	for {
		select {
		case operation := <-c.responseHandlers:
			switch value := operation.(type) {
			case registerResponseHandler:
				if _, ok := handlers[value.requestId]; ok {
					panic("attempting to register a handler for an already used request id")
				}
				handlers[value.requestId] = value.handler
			case removeResponseHandler:
				delete(handlers, value.requestId)
			}
		case m := <-c.responses:
			if handler, ok := handlers[m.response.RequestId]; ok {
				go handler.Handle(m.response, m.conn)
			} else {
				log.Println("got a response for a request with unknown id:", m.response.RequestId)
			}
		case <-c.done:
			return
		}
	}
}

func (c *client) outgoingBroadcastHandler() {
	for {
		var b broadcastPacket
		select {
		case b = <-c.broadcasts:
		case <-c.done:
			return
		}
		go func() {
			var buf bytes.Buffer
			n, err := b.packet.WriteTo(&buf)
			if n != b.packet.Size() {
				log.Printf("buffering of broadcast packet failed: only wrote %v of %v bytes\n", n, b.packet.Size())
				return
			}
			if err != nil {
				log.Println("failed to write a broadcast packet:", err)
				return
			}
			nets, _, err := c.tracker.Listen(b.ctx)
			for _, nw := range nets {
				if nw.IsBroadcast() && nw.IP.To4() != nil {
					// FIXME only supports IPv4 and broadcast, include multicast in the future
					e1 := c.beacon.Write(buf.Bytes(), nw)
					if e1 != nil {
						log.Printf("failed to write broadcast packet on %v: %v", nw.IP, e1)
						continue
					}
					ip, _ := nw.BroadcastIp()
					log.Println("IPV4-BROADCAST-SEND: request on broadcast address", ip)
				}
			}
			// TODO use the future channel to broadcast on new networks.
		}()
	}
}

func (c *client) incomingUnicastHandler() {
	l := log.New(log.Writer(), "incomingUnicastHandler: ", log.LstdFlags|log.Lmsgprefix)
	for {
		conn, err := c.listener.Accept()
		select {
		case <-c.done:
			return
		default:
		}
		if err != nil {
			if err == net.ErrClosed {
				return
			}
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			// TODO restart this on failure instead of exiting
			l.Fatalln("failed to accept:", err)
			return
		}
		go func() {
			success := false
			defer func() {
				if !success {
					conn.Close()
				}
			}()
			headerPacket, e1 := packet.DecodeFrom(conn)
			if e1 != nil {
				l.Println("failed to read packet:", e1)
				return
			}
			header := &message.Header{}
			e1 = proto.Unmarshal(headerPacket, header)
			if e1 != nil {
				l.Println("failed to parse header:", e1)
				return
			}
			contentPacket, e1 := packet.DecodeFrom(conn)
			if e1 != nil {
				l.Println("failed to read content packet:", e1)
				return
			}
			switch header.ConnectionType {
			default:
				l.Println("received invalid connection type in header:", e1)
				return
			case message.ConnectionTypeRequest:
				// ... TODO
			case message.ConnectionTypeResponse:
				responseMessage := &message.Response{}
				e1 = proto.Unmarshal(contentPacket, responseMessage)
				if e1 != nil {
					l.Println("failed to unmarshal response message:", e1)
					return
				}
				success = true
				c.responses <- responseConn{
					response: Response{
						RequestId:        RequestId(responseMessage.RequestId),
						HandshakeMessage: responseMessage.HandshakeMessage,
					},
					conn: conn,
				}
			}
		}()
	}
}

func (c *client) Connect(ctx context.Context, targetIdentity Identity, targetKey HMACKey, addrs ...string) (conn secure.Conn, err error) {
	e, err := c.cipherSuite.GenerateKeypair(rand.Reader)
	if err != nil {
		return
	}
	request := Request{
		Id:               c.nextRequestId(),
		HandshakeMessage: e.Public,
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	listener, err := c.request(ctx, targetKey, request)
	if err != nil {
		return
	}

	out := make(chan secure.Conn)
	outErr := make(chan error, 1)

	go func() {
		for {
			response, rawConn, e1 := listener.Accept()
			if e1 != nil {
				outErr <- e1
				cancel()
				return
			}
			secConn, e2 := c.handleResponse(e, targetIdentity, request, response, rawConn)
			if e2 != nil {
				log.Println("failed to handle response:", e2)
				rawConn.Close()
				continue
			}
			select {
			case out <- secConn:
			case <-ctx.Done():
				secConn.Close()
			}
			return
		}
	}()

	// TODO contact a device directly by using the addrs parameter.

	select {
	case conn = <-out:
	case <-ctx.Done():
		select {
		case err = <-outErr:
		default:
			err = ctx.Err()
		}
	case <-c.done:
		err = net.ErrClosed
	}
	return
}

func (c *client) Accept() (conn secure.Conn, err error) {
	// TODO do not allow this to be called multiple times

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listener, err := c.listenRequests(ctx)
	if err != nil {
		return
	}

	/// COPIED BEGIN
	/// FIXME this should not be copied
	out := make(chan secure.Conn)
	outErr := make(chan error, 1)

	go func() {
		for {
			request, rawConn, e1 := listener.Accept()
			if e1 != nil {
				outErr <- e1
				cancel()
				return
			}
			secConn, e2 := c.handleRequest(request, rawConn)
			if e2 != nil {
				log.Println("failed to handle request:", e2)
				rawConn.Close()
				continue
			}
			select {
			case out <- secConn:
			case <-ctx.Done():
				secConn.Close()
			}
			return
		}
	}()

	select {
	case conn = <-out:
	case <-ctx.Done():
		select {
		case err = <-outErr:
		default:
			err = ctx.Err()
		}
	case <-c.done:
		err = net.ErrClosed
	}
	return
	/// COPIED END
}

func (c *client) request(ctx context.Context, deviceKey HMACKey, request Request, addrs ...string) (listener ResponseListener, err error) {
	h, err := proto.Marshal(&message.Header{
		ConnectionType: message.ConnectionTypeRequest,
	})
	if err != nil {
		return
	}
	p, err := proto.Marshal(&message.RequestPayload{
		RequestId:        uint64(request.Id),
		EpochSeconds:     uint64(time.Now().Unix()),
		HandshakeMessage: request.HandshakeMessage,
	})
	if err != nil {
		return
	}
	hash := hmac.New(sha256.New, deviceKey)
	r, err := proto.Marshal(&message.Request{
		Payload: p,
		Mac:     hash.Sum(p),
	})
	if err != nil {
		return
	}

	listener, handler := NewResponseListener(ctx)
	c.responseHandlers <- registerResponseHandler{request.Id, handler}
	go func() {
		<-ctx.Done()
		c.responseHandlers <- removeResponseHandler{request.Id, handler}
	}()

	c.broadcasts <- broadcastPacket{ctx, packet.New(r)}
	_ = h
	return

	// outgoingBroadcastHandler:
	// - receives broadcast packets through the channel "broadcasts"
	// - sends those broadcasts using the "Beacon" type
	// - done with this packet

	// outgoingUnicastHandler: (TODO)
	// - receives unicast packets and the respective target address through the channel "unicasts"
	// - opens a tcp connection to the target address over tcp (4 or 6 implicitly)
	// - sends the packet as the first message (includes the header length, header, request length, request)
	// - waits for a Response message on this connection
	// THEN: c.responses <- ResponseConn{response, conn}
}

func (c *client) listenRequests(ctx context.Context) (listener RequestListener, err error) {
	listener, handler := NewRequestListener(ctx)
	l := log.New(log.Writer(), "listenRequests: ", log.LstdFlags|log.Lmsgprefix)

	type packetAddr struct {
		data []byte
		addr net.Addr
	}
	// buffer a small amount to reduce the likelihood of missing packets
	packets := make(chan packetAddr, 8)
	go func() {
		for {
			data, addr, e1 := c.beacon.Read()
			if e1 != nil {
				// TODO restart instead of bailing out
				l.Println("failed to read from beacon:", e1)
				return
			}
			if ctx.Err() != nil {
				return
			}
			p, e1 := packet.Decode(data)
			if e1 != nil {
				l.Println("failed to decode packet:", e1)
				continue
			}
			packets <- packetAddr{p, addr}
		}
	}()

	go func() { // broadcast
		for {
			var p packetAddr
			select {
			case p = <-packets:
			case <-ctx.Done():
				return
			}
			requestMessage := &message.Request{}
			e1 := proto.Unmarshal(p.data, requestMessage)
			if e1 != nil {
				fmt.Println("RECEIVED:", len(p.data), p.data) // hex.EncodeToString(sha256.New().Sum(p.data)))
				l.Println("failed to unmarshal request from beacon:", e1)
				continue
			}
			request, e1 := c.verifyRequestMessage(c.deviceKey, requestMessage)
			if e1 != nil {
				l.Println("failed to verify request message", e1)
				continue
			}
			addr, ok := p.addr.(*net.UDPAddr)
			if !ok {
				l.Println("address is not a udp address")
				continue
			}
			conn, e1 := c.dialer.DialContext(ctx, "tcp", (&net.TCPAddr{IP: addr.IP, Port: c.port}).String())
			if e1 != nil {
				l.Printf("failed to dial %s for request id %v: %v", p.addr, request.Id, e1)
				continue
			}
			h := &message.Header{ConnectionType: message.ConnectionTypeResponse}
			header, e1 := proto.Marshal(h)
			if e1 != nil {
				l.Println("failed to marshal response header:", e1)
				continue
			}
			_, e1 = packet.New(header).WriteTo(conn)
			if e1 != nil {
				l.Println("failed to write response header to connection:", e1)
				continue
			}
			if !handler.Handle(request, conn) {
				l.Printf("request with id %v was not handled", request.Id)
				conn.Close()
				return
			}
		}
	}()

	return

	// read beacon packets
	// -> verify packet
	// -> dial the address in the packet
	// -> call handleRequest with the request from the beacon packet and the established connection

	// accept tcp connections (TODO)
	// -> read the header
	// -> IF request:
	//    - next packet is a Request
	//    - verify the request message
	//    - if error: log and close connection
	//    - otherwise: handle request with parsed request and connection
	// -> IF response:
	//    - write the response and the conn to the responses channel which is then handled by the broker
}

func (c *client) verifyRequestMessage(deviceKey HMACKey, requestMessage *message.Request) (request Request, err error) {
	hash := hmac.New(sha256.New, deviceKey)
	mac := hash.Sum(requestMessage.Payload)
	if !bytes.Equal(mac, requestMessage.Mac) {
		err = errors.New("the device key was not used for this request message")
		return
	}
	payload := &message.RequestPayload{}
	err = proto.Unmarshal(requestMessage.Payload, payload)
	if err != nil {
		err = fmt.Errorf("failed to parse payload: %w", err)
		return
	}
	age := time.Since(time.Unix(int64(payload.EpochSeconds), 0)) // FIXME possible overflow
	if age > RequestMaxAge {
		err = errors.New("request message is too old")
		return
	}
	request = Request{
		Id:               RequestId(payload.RequestId),
		HandshakeMessage: payload.HandshakeMessage,
	}
	return
}

// handleResponse handles a response from a device.
// It takes a Response instance with the data that was sent by the responding device
// and a generic net.Conn for communicating with the device.
// The connection is expected to be in a state where the next message sent by us is a message.Hello.
func (c *client) handleResponse(ephemeralKey NoiseKeyPair, targetIdentity Identity,
	request Request, response Response, conn net.Conn) (noiseConn secure.Conn, err error) {
	hs, err := c.requesterHandshakeState(request, ephemeralKey, targetIdentity)
	if err != nil {
		return
	}
	_, _, _, err = hs.ReadMessage(nil, response.HandshakeMessage)
	if err != nil {
		return
	}

	// Write Hello.
	hs1, c1, c2, err := hs.WriteMessage(nil, nil)
	if err != nil {
		return
	}
	m1 := &message.Hello{HandshakeMessage: hs1}
	d1, err := proto.Marshal(m1)
	if err != nil {
		return
	}
	err = conn.SetWriteDeadline(time.Now().Add(IoTimeout))
	if err != nil {
		return
	}
	_, err = packet.New(d1).WriteTo(conn)
	if err != nil {
		return
	}
	err = conn.SetWriteDeadline(time.Time{})
	if err != nil {
		return
	}

	// Wrap the connection.
	noiseConn = secure.WrapConn(c.Identity(), targetIdentity, conn, c2, c1)

	// Read the encrypted Result.
	err = noiseConn.SetReadDeadline(time.Now().Add(IoTimeout))
	if err != nil {
		return
	}
	buf := make([]byte, secure.MessageMaxSize) // TODO allow smaller buffer sizes with secure.Conn.Read.
	n, err := noiseConn.Read(buf)
	if err != nil {
		return
	}
	err = noiseConn.SetReadDeadline(time.Time{})
	if err != nil {
		return
	}
	data := buf[:n]
	result := &message.Result{}
	err = proto.Unmarshal(data, result)
	if err != nil {
		return
	}
	if result.Type == message.ResultTypeRefused {
		err = ConnectionRefused
		return
	}
	if result.Type != message.ResultTypeOk {
		err = errors.New("invalid result type in result message")
		return
	}
	return
}

// handleRequest handles a request from a device.
// It takes a Request instance with the data that was sent by the requesting device
// and a generic net.Conn for communicating with the device.
// The connection is expected to be in a state where the next message sent by us is a message.Response.
func (c *client) handleRequest(request Request, conn net.Conn) (noiseConn secure.Conn, err error) {
	hs, err := c.responderHandshakeState(request)
	if err != nil {
		return
	}

	// Write Response.
	hs1, _, _, err := hs.WriteMessage(nil, nil)
	if err != nil {
		return
	}
	m1 := &message.Response{
		RequestId:        uint64(request.Id),
		HandshakeMessage: hs1,
	}
	b1, err := proto.Marshal(m1)
	if err != nil {
		return
	}
	_, err = packet.New(b1).WriteTo(conn)
	if err != nil {
		return
	}

	// Read Hello.
	err = conn.SetReadDeadline(time.Now().Add(IoTimeout))
	if err != nil {
		return
	}
	d2, err := packet.DecodeFrom(conn)
	if err != nil {
		return
	}
	m2 := &message.Hello{}
	err = proto.Unmarshal(d2, m2)
	if err != nil {
		return
	}
	_, c1, c2, err := hs.ReadMessage(nil, m2.HandshakeMessage)
	if err != nil {
		return
	}

	peerStatic := X25519Identity(hs.PeerStatic())
	if peerStatic == nil {
		err = errors.New("unexpected nil peer static identity")
		return
	}
	// FIXME: peerStatic is an X25519 identity, but an Ed25519 identity is expected.
	//  Usually one would check all the trusted identities
	//  by converting each to the X25519-equivalent, comparing it with peerStatic
	//  and then using the Ed25519 public key.
	peerIdentity := Identity(peerStatic)
	// Wrap the connection.
	// The result is the first encrypted message.
	noiseConn = secure.WrapConn(c.Identity(), peerIdentity, conn, c1, c2)

	// Check if the identity is trusted.
	if !c.trustStore.IsX25519Trusted(peerStatic) {
		err = c.writeResult(noiseConn, message.ResultTypeRefused)
		return
	}

	// Write Result.
	err = c.writeResult(noiseConn, message.ResultTypeOk)
	if err != nil {
		return
	}
	return
}

func (c *client) writeResult(conn net.Conn, resultType message.ResultType) (err error) {
	err = conn.SetWriteDeadline(time.Now().Add(IoTimeout))
	if err != nil {
		return
	}
	data, err := proto.Marshal(&message.Result{Type: resultType})
	if err != nil {
		return
	}
	_, err = conn.Write(data)
	if err != nil {
		return err
	}
	err = conn.SetWriteDeadline(time.Time{})
	return
}

func (c *client) requesterHandshakeState(request Request, ephemeral NoiseKeyPair, responderIdentity Identity) (*HandshakeState, error) {
	return NewHandshakeState(Config{
		CipherSuite:      c.cipherSuite,
		Random:           rand.Reader,
		Pattern:          secure.HandshakeXK1fallback,
		Initiator:        false,
		Prologue:         request.Id.Bytes(),
		StaticKeypair:    c.keyPair.Noise(),
		EphemeralKeypair: ephemeral,
		PeerStatic:       responderIdentity.X25519(),
	})
}

func (c *client) responderHandshakeState(request Request) (*HandshakeState, error) {
	return NewHandshakeState(Config{
		CipherSuite:   c.cipherSuite,
		Random:        rand.Reader,
		Pattern:       secure.HandshakeXK1fallback,
		Initiator:     true,
		Prologue:      request.Id.Bytes(),
		StaticKeypair: c.keyPair.Noise(),
		PeerEphemeral: request.HandshakeMessage,
	})
}

func (c *client) nextRequestId() RequestId {
	// FIXME a sequential id reveals to another party
	//  how many requests we have sent since this client is active.
	//  it should be random.
	return RequestId(atomic.AddUint64(&c.requestId, 1))
}

/*

client/
	Client/
		Dial(ctx context.Context, target HmacKey, identity LocalIdentity)
		-> noise.Conn, error
		Accept(self HmacKey)
		-> noise.Conn, error

network/
	Network/
		Search(request, addrs...) -- initiates a request with a specific device (writes beacon messages and opens tcp connections)
		-> *ResponseListener, error
		Accept() -- accepts requests of any device (reads beacon messages and accepts tcp connections)
		-> *RequestListener, error

listener/
	NewListener(ctx context.Context, out chan interface{})
	Listener/
		Accept()
		-> interface{}, net.Conn, error
		Done() <-chan struct{}
		Close()
		error(err error) -- reports an error and delivers it to Accept calls, this function does not block

request/
	Request
	NewRequestListener(ctx context.Context, out chan interface{}, errs chan error)
	RequestListener : Listener/
		Accept()
		-> *Request, net.Conn, error

response/
	Response
	ResponseListener : Listener/
		Accept()
		-> *Response, net.Conn, error

future:

client/
	Announce() -- announces this device on the network (broadcast / multicast)
	Wait()    -- waits for the Announce() of a device with a specific key
	Connect() -- similar to Search() but used after a call to Wait() to connect to the device

*/
