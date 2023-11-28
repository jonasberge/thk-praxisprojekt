package local

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/flynn/noise"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
	"io"
	mathRand "math/rand"
	"net"
	"projekt/core/lib/beacon"
	"projekt/core/lib/device"
	"projekt/core/lib/local/message"
	"projekt/core/lib/network"
	"projekt/core/lib/packet"
	"projekt/core/lib/secure"
	"sync"
	"testing"
	"time"
)

const (
	DeviceKeySize = 32
	Port          = 23520
)

var cs = noise.NewCipherSuite(noise.DH25519, noise.CipherAESGCM, noise.HashSHA256)

func createKeyPair(t *testing.T) device.KeyPair {
	key, err := device.GenerateKeyPair(rand.Reader)
	assert.Nil(t, err)
	return key
}

func createDeviceKey(t *testing.T) device.HMACKey {
	key := make([]byte, DeviceKeySize)
	n, err := rand.Reader.Read(key)
	assert.Nil(t, err)
	assert.Equal(t, len(key), n)
	return key
}

func createPredictableIdentityKey(t *testing.T, seed int64) device.KeyPair {
	reader := mathRand.New(mathRand.NewSource(seed))
	key, err := device.GenerateKeyPair(reader)
	assert.Nil(t, err)
	return key
}

func createPredictableDeviceKey(t *testing.T, seed int64) device.HMACKey {
	key := make([]byte, DeviceKeySize)
	reader := mathRand.New(mathRand.NewSource(seed))
	n, err := reader.Read(key)
	assert.Nil(t, err)
	assert.Equal(t, len(key), n)
	return key
}

type simpleTrustStore struct {
	trusted []device.Identity
}

func (t *simpleTrustStore) IsTrusted(identity device.Identity) bool {
	for _, trusted := range t.trusted {
		if bytes.Equal(trusted, identity) {
			return true
		}
	}
	return false
}

func (t *simpleTrustStore) IsX25519Trusted(identity device.X25519Identity) bool {
	for _, trusted := range t.trusted {
		if bytes.Equal(trusted.X25519(), identity) {
			return true
		}
	}
	return false
}

func createTrusted(trusted ...device.Identity) TrustStore {
	return &simpleTrustStore{trusted}
}

func createClient(t *testing.T) *client {
	identityKey := createKeyPair(t)
	deviceKey := createDeviceKey(t)
	return NewClient(Port, identityKey, deviceKey, createTrusted(), cs).(*client)
}

func createTrustingClient(t *testing.T, trusted ...device.Identity) *client {
	identityKey := createKeyPair(t)
	deviceKey := createDeviceKey(t)
	return NewClient(Port, identityKey, deviceKey, createTrusted(trusted...), cs).(*client)
}

func createPredictableClient(t *testing.T, seed int64) *client {
	identityKey := createPredictableIdentityKey(t, seed)
	deviceKey := createPredictableDeviceKey(t, seed)
	return NewClient(Port, identityKey, deviceKey, createTrusted(), cs).(*client)
}

func createPredictableTrustingClient(t *testing.T, seed int64, trusted ...device.Identity) *client {
	identityKey := createPredictableIdentityKey(t, seed)
	deviceKey := createPredictableDeviceKey(t, seed)
	return NewClient(Port, identityKey, deviceKey, createTrusted(trusted...), cs).(*client)
}

func createEphemeralKeypair(t *testing.T) noise.DHKey {
	e, err := cs.GenerateKeypair(rand.Reader)
	assert.Nil(t, err)
	return e
}

func readMessage(t *testing.T, conn net.Conn, message proto.Message) proto.Message {
	assert.Nil(t, conn.SetReadDeadline(time.Now().Add(100*time.Millisecond)))
	//buf := make([]byte, 1<<10)
	//n, err := conn.Read(buf)
	data, err := packet.DecodeFrom(conn)
	assert.Nil(t, err)
	assert.Nil(t, conn.SetReadDeadline(time.Time{}))
	//data := buf[:n]
	err = proto.Unmarshal(data, message)
	assert.Nil(t, err)
	return message
}

func encConnWrite(t *testing.T, conn secure.Conn, data []byte) {
	n, err := conn.Write(data)
	assert.Nil(t, err)
	assert.Equal(t, len(data), n)
}

func encConnReadExpect(t *testing.T, conn secure.Conn, expected []byte) {
	readBuf := make([]byte, secure.MessageMaxSize)
	n, err := conn.Read(readBuf)
	assert.Nil(t, err)
	assert.Equal(t, len(expected), n)
	assert.EqualValues(t, expected, readBuf[:n])
}

func testEncryptedConnPair(t *testing.T, c1 secure.Conn, c2 secure.Conn) {
	var wg sync.WaitGroup
	secret := []byte("maritime")

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 64; i++ {
			encConnWrite(t, c1, secret)
			encConnReadExpect(t, c1, secret)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 64; i++ {
			encConnReadExpect(t, c2, secret)
			encConnWrite(t, c2, secret)
		}
	}()

	wg.Wait()
}

type TeeConn struct {
	net.Conn
	io.ReadWriter
	name  string
	muted bool
}

func (c *TeeConn) Read(p []byte) (n int, err error) {
	n, err = c.Conn.Read(p)
	if c.muted {
		return
	}
	if n > 0 {
		fmt.Printf("[%s] recv %v: %v\n", c.name, n, p[:n])
	}
	if err != nil {
		fmt.Printf("[%s] recv err: %v\n", c.name, err)
	}
	return
}

func (c *TeeConn) Write(p []byte) (n int, err error) {
	n, err = c.Conn.Write(p)
	if c.muted {
		return
	}
	if n > 0 {
		fmt.Printf("[%s] send %v: %v\n", c.name, len(p), p)
	}
	if err != nil {
		fmt.Printf("[%s] send err: %v\n", c.name, err)
	}
	return
}

func (c *TeeConn) Mute() {
	c.muted = true
}

func TestClient_handleRequest_handleResponse(t *testing.T) {
	initiator := createClient(t)
	responder := createTrustingClient(t, initiator.Identity())

	e := createEphemeralKeypair(t)
	request := Request{
		Id:               RequestId(1),
		HandshakeMessage: e.Public,
	}

	var wg errgroup.Group
	conn, responderConn := net.Pipe()
	connCh := make(chan secure.Conn, 2)

	wg.Go(func() error {
		encConn, err := responder.handleRequest(request, responderConn)
		assert.Nil(t, err)
		connCh <- encConn
		return err
	})

	wg.Go(func() error {
		responseMessage := readMessage(t, conn, &message.Response{}).(*message.Response)
		response := Response{RequestId: request.Id, HandshakeMessage: responseMessage.HandshakeMessage}
		encConn, err := initiator.handleResponse(e, responder.Identity(), request, response, conn)
		assert.Nil(t, err)
		connCh <- encConn
		return err
	})

	err := wg.Wait()
	if err != nil {
		t.FailNow()
	}

	testEncryptedConnPair(t, <-connCh, <-connCh)
}

func newDummyTracker(nw network.Net) network.Tracker {
	return &dummyTracker{
		network: nw,
	}
}

type dummyTracker struct {
	network network.Net
}

func (t dummyTracker) Listen(ctx context.Context) (present []network.Net, future <-chan *network.Net, err error) {
	present = []network.Net{t.network}
	out := make(chan *network.Net)
	close(out)
	future = out
	return
}

type DummyNetwork interface {
	beacon.Beacon
	net.Listener
	Dialer
}

type dummyNetwork struct {
	network    network.Net
	localAddr  net.Addr
	remoteAddr net.Addr
	conn       net.Conn
	connBeacon net.Conn
	localDial  chan struct{}
	remoteDial chan struct{}
}

func newDummyNetworkPair(nw network.Net, localAddr net.Addr, remoteAddr net.Addr) (net.Conn, net.Conn, DummyNetwork, DummyNetwork) {
	local, remote := net.Pipe()
	localBeacon, remoteBeacon := net.Pipe()
	localDial := make(chan struct{})
	remoteDial := make(chan struct{})
	localNet := &dummyNetwork{
		network:    nw,
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
		conn:       local,
		connBeacon: localBeacon,
		localDial:  localDial,
		remoteDial: remoteDial,
	}
	remoteNet := &dummyNetwork{
		network:    nw,
		localAddr:  remoteAddr,
		remoteAddr: localAddr,
		conn:       remote,
		connBeacon: remoteBeacon,
		localDial:  remoteDial,
		remoteDial: localDial,
	}
	return local, remote, localNet, remoteNet
}

func (t dummyNetwork) Listen() error { return nil }
func (t dummyNetwork) Close() error  { return nil }

func (t dummyNetwork) Addr() net.Addr {
	return t.localAddr
}

func (t dummyNetwork) Write(data []byte, dst network.Net) (err error) {
	if dst.String() != t.network.String() {
		panic("written to wrong network")
	}
	fmt.Println("TEST-NETWORK: writing to", dst.String(), "-", data)
	n, err := t.connBeacon.Write(data)
	fmt.Println("TEST-NETWORK: written to", dst.String(), "-", data)
	if err != nil {
		return
	}
	if n != len(data) {
		err = errors.New("failed to write entire data")
	}
	return
}

func (t dummyNetwork) Read() (data []byte, addr net.Addr, err error) {
	fmt.Println("TEST-NETWORK: reading")
	const Size = 1024
	buf := make([]byte, Size)
	n, err := t.connBeacon.Read(buf)
	if err != nil {
		return
	}
	if n == Size {
		panic("buffer is too small")
	}
	data = buf[:n]
	addr = t.remoteAddr
	fmt.Println("TEST-NETWORK: read from", addr, "-", data)
	return
}

func (t dummyNetwork) Accept() (net.Conn, error) {
	fmt.Println("TEST-NETWORK: accepting conn as", t.Addr(), "- waiting for dial...")
	<-t.remoteDial
	fmt.Println("TEST-NETWORK: accepted conn as", t.Addr(), "to remote")
	return t.conn, nil
}

func (t dummyNetwork) DialContext(ctx context.Context, network string, address string) (net.Conn, error) {
	if network != "tcp" {
		panic("network must be tcp")
	}
	if address != t.remoteAddr.String() {
		panic("invalid address used in DialContext: " + address)
	}
	fmt.Println("dialing", network, address, "as", t.Addr())
	t.localDial <- struct{}{}
	fmt.Println("dial was accepted")
	return t.conn, nil
}

type dummies struct {
	net        network.Net
	tracker    network.Tracker
	localAddr  net.Addr
	localConn  net.Conn
	localNet   DummyNetwork
	remoteAddr net.Addr
	remoteConn net.Conn
	remoteNet  DummyNetwork
}

func (d dummies) Tracker() network.Tracker {
	return d.tracker
}

func (d dummies) LocalDialer() DummyNetwork {
	return d.localNet
}

func (d dummies) RemoteDialer() DummyNetwork {
	return d.remoteNet
}

func (d dummies) LocalBeacon() beacon.Beacon {
	return d.localNet
}

func (d dummies) LocalListener() net.Listener {
	return d.localNet
}

func (d dummies) RemoteBeacon() beacon.Beacon {
	return d.remoteNet
}

func (d dummies) RemoteListener() net.Listener {
	return d.remoteNet
}

func createDummies(t *testing.T) *dummies {
	dummyNet := network.Net{
		IPNet: net.IPNet{
			IP:   net.ParseIP("123.45.67.89"),
			Mask: net.IPv4Mask(255, 255, 255, 0),
		},
		Interface: net.Interface{
			Index: 100,
			Flags: net.FlagUp | net.FlagBroadcast,
		},
	}
	tracker := newDummyTracker(dummyNet)
	localAddr := &net.UDPAddr{IP: net.ParseIP("100.0.0.1"), Port: Port}
	remoteAddr := &net.UDPAddr{IP: net.ParseIP("100.0.0.2"), Port: Port}
	localConn, remoteConn, localNet, remoteNet := newDummyNetworkPair(dummyNet, localAddr, remoteAddr)
	assert.Nil(t, localConn.SetDeadline(time.Time{}))
	assert.Nil(t, remoteConn.SetDeadline(time.Time{}))
	return &dummies{
		net:        dummyNet,
		tracker:    tracker,
		localAddr:  localAddr,
		localConn:  localConn,
		localNet:   localNet,
		remoteAddr: remoteAddr,
		remoteConn: remoteConn,
		remoteNet:  remoteNet,
	}
}

func TestDummies(t *testing.T) {
	d := createDummies(t)

	nets, _, err := d.tracker.Listen(context.TODO())
	assert.Nil(t, err)
	assert.Equal(t, d.net, nets[0])
	assert.EqualValues(t, 1, len(nets))

	data := "fizzbuzz"
	assert.Panics(t, func() {
		_ = d.localNet.Write([]byte(data), network.Net{})
	})

	type dummyNetworkPair struct {
		src     DummyNetwork
		dst     DummyNetwork
		srcAddr net.Addr
	}
	routes := []dummyNetworkPair{
		{src: d.localNet, dst: d.remoteNet, srcAddr: d.localAddr},
		{src: d.remoteNet, dst: d.localNet, srcAddr: d.remoteAddr},
	}
	for _, route := range routes {
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			err = route.src.Write([]byte(data), d.net)
			assert.Nil(t, err)
		}()
		go func() {
			defer wg.Done()
			buf, addr, err := route.dst.Read()
			assert.Nil(t, err)
			assert.EqualValues(t, route.srcAddr, addr)
			assert.EqualValues(t, data, string(buf))
		}()
		wg.Wait()
	}

	type dummyDialerListener struct {
		dialer     Dialer
		listener   net.Listener
		remoteAddr net.Addr
		dialerConn net.Conn
		listenConn net.Conn
	}
	dialRoutes := []dummyDialerListener{
		{dialer: d.LocalDialer(), listener: d.RemoteListener(), remoteAddr: d.remoteAddr, dialerConn: d.localConn, listenConn: d.remoteConn},
		{dialer: d.RemoteDialer(), listener: d.LocalListener(), remoteAddr: d.localAddr, dialerConn: d.remoteConn, listenConn: d.localConn},
	}
	for _, route := range dialRoutes {
		sleepTime := 50 * time.Millisecond
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			time.Sleep(sleepTime)
			conn, err := route.dialer.DialContext(context.TODO(), "tcp", route.remoteAddr.String())
			assert.Nil(t, err)
			assert.Equal(t, route.dialerConn, conn)
		}()
		go func() {
			defer wg.Done()
			start := time.Now()
			conn, err := route.listener.Accept()
			delta := time.Since(start)
			assert.GreaterOrEqual(t, delta, sleepTime)
			assert.Nil(t, err)
			assert.Equal(t, route.listenConn, conn)
		}()
		wg.Wait()
	}
}

func TestClient_Dummy_Connect_Accept(t *testing.T) {
	initiator := createClient(t)
	responder := createTrustingClient(t, initiator.Identity())

	fmt.Println("initiator-identity:", initiator.Identity()) // hex.EncodeToString(initiator.Identity()))
	fmt.Println("responder-identity:", responder.Identity()) // hex.EncodeToString(initiator.Identity()))

	d := createDummies(t)
	initiator.testListen(d.Tracker(), d.LocalBeacon(), d.LocalListener(), d.LocalDialer())
	responder.testListen(d.Tracker(), d.RemoteBeacon(), d.RemoteListener(), d.RemoteDialer())

	ctx := context.Background()
	connCh := make(chan secure.Conn, 2)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		conn, err := initiator.Connect(ctx, responder.Identity(), responder.HmacKey())
		assert.Nil(t, err)
		connCh <- conn
	}()
	go func() {
		defer wg.Done()
		conn, err := responder.Accept()
		assert.Nil(t, err)
		connCh <- conn
	}()
	wg.Wait()

	testEncryptedConnPair(t, <-connCh, <-connCh)
}
