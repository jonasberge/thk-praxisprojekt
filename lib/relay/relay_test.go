package relay

import (
	"bytes"
	. "context"
	"crypto/rand"
	. "github.com/flynn/noise"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
	mathRand "math/rand"
	"net"
	. "projekt/core/lib/device"
	"projekt/core/lib/secure"
	"sync"
	"testing"
	"time"
)

const SessionPort = 23521

var (
	cs  = NewCipherSuite(DH25519, CipherAESGCM, HashSHA256)
	ad1 = []byte("foo")
	ad2 = []byte("bar")
)

func createIdentity(t *testing.T) DHKey {
	keypair, err := cs.GenerateKeypair(rand.Reader)
	assert.Nil(t, err)
	return keypair
}

func createServerClients(t *testing.T) (*ServerProtocol, *ClientProtocol, *ClientProtocol, Identity, Identity, Context, CancelFunc) {
	ctx, cancel := createContext()
	id1 := createIdentity(t)
	id2 := createIdentity(t)
	cc1, sc1 := net.Pipe()
	cc2, sc2 := net.Pipe()
	sp := NewServerProtocol(SessionPort)
	assert.Nil(t, sp.Start())
	sp.AddClient(id1.Public, sc1)
	sp.AddClient(id2.Public, sc2)
	cp1 := NewClientProtocol(cc1)
	cp2 := NewClientProtocol(cc2)
	assert.Nil(t, cp1.Start(ctx))
	assert.Nil(t, cp2.Start(ctx))
	return sp, cp1, cp2, id1.Public, id2.Public, ctx, func() {
		cancel()
		assert.Nil(t, cp1.Close())
		assert.Nil(t, cp2.Close())
		assert.Nil(t, sp.Close())
	}
}

func createContext() (ctx Context, cancel func()) {
	return Background(), func() {} // WithTimeout(Background(), 100*time.Millisecond)
}

func TestProtocol_TwoClients(t *testing.T) {
	_, _, _, _, _, _, cancel := createServerClients(t)
	defer cancel()
}

func TestProtocol_AlreadyConnected(t *testing.T) {
	ctx, cancel := createContext()
	defer cancel()
	id := createIdentity(t)
	cc1, sc1 := net.Pipe()
	cc2, sc2 := net.Pipe()
	sp := NewServerProtocol(SessionPort)
	assert.Nil(t, sp.Start())
	sp.AddClient(id.Public, sc1)
	sp.AddClient(id.Public, sc2)
	cp1 := NewClientProtocol(cc1)
	assert.Nil(t, cp1.Start(ctx))
	cp2 := NewClientProtocol(cc2)
	err := cp2.Start(ctx)
	assert.ErrorIs(t, err, ErrAlreadyConnected)
}

func Test_ServerProtocol_ClientProtocol(t *testing.T) {
	sp, cp1, cp2, id1, id2, ctx, cancel := createServerClients(t)
	defer cancel()

	infos := make(chan SessionInfo, 2)
	ready := make(chan struct{})

	var e errgroup.Group
	e.Go(func() (err error) {
		close(ready)
		info, ad, err := cp2.ReadInvitation()
		assert.Nil(t, err)
		assert.EqualValues(t, id1, info.PeerIdentity)
		assert.EqualValues(t, SessionPort, info.Description.Port)
		assert.EqualValues(t, ad1, ad)
		err = cp2.Accept(info.PeerIdentity, ad2)
		assert.Nil(t, err)
		infos <- info
		return
	})
	e.Go(func() (err error) {
		<-ready
		time.Sleep(25 * time.Millisecond)
		info, err := cp1.Request(ctx, id2, ad1)
		assert.Nil(t, err)
		assert.EqualValues(t, id2, info.PeerIdentity)
		assert.EqualValues(t, SessionPort, info.Description.Port)
		ad, err := cp1.ReadAccept(ctx, info.PeerIdentity)
		assert.Nil(t, err)
		assert.EqualValues(t, ad2, ad)
		infos <- info
		return
	})

	assert.Nil(t, e.Wait())
	testServerProtocolClientSessionProtocol(t, sp, <-infos, <-infos)
}

func testServerProtocolClientSessionProtocol(t *testing.T, sp *ServerProtocol, i1, i2 SessionInfo) {
	sc1, cc1 := net.Pipe()
	sc2, cc2 := net.Pipe()
	sp.AddSessionClient(sc1)
	sp.AddSessionClient(sc2)
	cp1 := NewSessionClientProtocol(cc1)
	cp2 := NewSessionClientProtocol(cc2)
	c1, err := cp1.Authenticate(i1.Description.Key)
	assert.Nil(t, err)
	c2, err := cp2.Authenticate(i2.Description.Key)
	assert.Nil(t, err)
	testConnectionPair(t, c1, c2)
}

func testConnectionPair(t *testing.T, c1, c2 net.Conn) {
	testConnectionPairUnidirectional(t, c1, c2)
	testConnectionPairUnidirectional(t, c2, c1)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func testConnectionPairUnidirectional(t *testing.T, c1, c2 net.Conn) {
	// FIXME There's a duplicate implementation in a test of the local client.
	const count = 128
	var seed = mathRand.Int63()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		buf := make([]byte, max(count, secure.PayloadMaxSize)) // account for secure.Conn
		reader := mathRand.New(mathRand.NewSource(seed))
		for i := 1; i < count; i++ {
			n, err := reader.Read(buf[:i])
			assert.Nil(t, err)
			assert.EqualValues(t, i, n)
			m, err := c1.Write(buf[:i])
			assert.Nil(t, err)
			assert.EqualValues(t, i, m)
		}
	}()
	go func() {
		defer wg.Done()
		expected := make([]byte, max(count, secure.PayloadMaxSize)) // account for secure.Conn
		actual := make([]byte, max(count, secure.PayloadMaxSize))
		reader := mathRand.New(mathRand.NewSource(seed))
		for i := 1; i < count; i++ {
			n, err := reader.Read(expected[:i])
			assert.Nil(t, err)
			assert.EqualValues(t, i, n)
			m, err := c2.Read(actual)
			assert.Nil(t, err)
			assert.EqualValues(t, i, m)
			assert.EqualValues(t, expected[:i], actual[:i])
		}
	}()
	wg.Wait()
}

func Test_Server_Client(t *testing.T) {
	key, err := GenerateKeyPair(rand.Reader)
	assert.Nil(t, err)
	cert, err := key.Certificate()
	assert.Nil(t, err)

	s := NewServer(cert, 23520, 23521)
	assert.Nil(t, s.Listen())

	k1, err := GenerateKeyPair(rand.Reader)
	assert.Nil(t, err)
	c1, err := Dial(k1, "127.0.0.1", "23520")
	assert.Nil(t, err)

	k2, err := GenerateKeyPair(rand.Reader)
	assert.Nil(t, err)
	c2, err := Dial(k2, "127.0.0.1", "23520")
	assert.Nil(t, err)

	conns := make(chan net.Conn, 2)

	var e errgroup.Group
	e.Go(func() (err error) {
		conn, err := c1.Accept(&TrustOne{identity: k2.Public})
		assert.Nil(t, err)
		assert.EqualValues(t, k1.Public, conn.LocalIdentity())
		assert.EqualValues(t, k2.Public, conn.RemoteIdentity())
		conns <- conn
		return err
	})
	e.Go(func() (err error) {
		time.Sleep(10 * time.Millisecond)
		conn, err := c2.Request(context.TODO(), k1.Public)
		assert.Nil(t, err)
		assert.EqualValues(t, k2.Public, conn.LocalIdentity())
		assert.EqualValues(t, k1.Public, conn.RemoteIdentity())
		conns <- conn
		return err
	})

	err = e.Wait()
	if err != nil {
		t.FailNow()
	}
	testConnectionPair(t, <-conns, <-conns)
}

type TrustOne struct {
	identity Identity
}

func (t *TrustOne) IsTrusted(identity Identity) bool {
	return bytes.Equal(t.identity, identity)
}

func (t *TrustOne) IsX25519Trusted(identity X25519Identity) bool {
	return bytes.Equal(t.identity.X25519(), identity)
}
