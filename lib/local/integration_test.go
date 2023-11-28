//+build container

package local

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/nettest"
	"log"
	"math/rand"
	"net"
	"projekt/core/lib/network"
	"projekt/core/lib/secure"
	"projekt/core/lib/util/container"
	"testing"
	"time"
)

// Start the containers before running tests:
// $ sudo make container_start

const (
	port = 23520
	data = "something useful"
)

var alpha = container.New("alpha")
var bravo = container.New("bravo")

func getNet(t *testing.T) network.Net {
	// TODO this function is duplicated
	in, err := nettest.RoutedInterface("ip4", net.FlagUp|net.FlagBroadcast)
	assert.Nil(t, err)
	addrs, err := in.Addrs()
	assert.Nil(t, err)
	var addr *net.IPNet
	for _, a := range addrs {
		n, ok := a.(*net.IPNet)
		assert.True(t, ok)
		if n.IP.To4() != nil {
			addr = n
			break
		}
	}
	if addr == nil {
		log.Fatal("no suitable IPv4 network connected")
	}
	return network.Net{
		IPNet:     *addr,
		Interface: *in,
	}
}

const testConnCount = 128

func testConnSender(t *testing.T, conn net.Conn) {
	reader := rand.New(rand.NewSource(0))
	total := 0
	for i := 1; i < testConnCount; i++ {
		out := make([]byte, i)
		n, err := reader.Read(out)
		assert.Nil(t, err)
		assert.EqualValues(t, i, n)
		m, err := conn.Write(out)
		assert.Nil(t, err)
		assert.EqualValues(t, i, m)
		total += i
	}
	fmt.Printf("Successfully transmitted %v random bytes to remote\n", total)
}

func testConnReceiver(t *testing.T, conn net.Conn) {
	reader := rand.New(rand.NewSource(0))
	in := make([]byte, secure.MessageMaxSize)
	total := 0
	for i := 1; i < testConnCount; i++ {
		expected := make([]byte, i)
		n, err := reader.Read(expected)
		assert.Nil(t, err)
		assert.EqualValues(t, i, n)
		m, err := conn.Read(in)
		assert.Nil(t, err)
		assert.EqualValues(t, i, m)
		assert.EqualValues(t, expected, in[:m])
		total += i
	}
	fmt.Printf("Successfully read transmission of %v random bytes from remote\n", total)
}

/* $ sudo go run test.go local TestClient_Connect_Accept */
func TestClient_Connect_Accept(t *testing.T) {
	initiator := createPredictableClient(t, 0)
	responder := createPredictableTrustingClient(t, 1, initiator.Identity())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	group := container.NewGroup(alpha, bravo)
	alpha.Run(func() {
		err := initiator.Listen()
		assert.Nil(t, err)
		fmt.Println("identity:", initiator.Identity().X25519())
		alpha.Sync(group)

		log.Println("Connecting (alpha)...")
		conn, err := initiator.Connect(ctx, responder.Identity(), responder.HmacKey())
		assert.Nil(t, err)
		fmt.Println("remote identity:", conn.RemoteIdentity())
		testConnSender(t, conn)
		testConnReceiver(t, conn)
	})
	bravo.RunAsync(func() {
		err := responder.Listen()
		assert.Nil(t, err)
		fmt.Println("identity:", responder.Identity())
		go func() {
			defer bravo.Done()
			log.Println("Accepting (bravo)...")
			conn, err := responder.Accept()
			assert.Nil(t, err)
			fmt.Println("remote identity:", conn.RemoteIdentity())
			testConnReceiver(t, conn)
			testConnSender(t, conn)
		}()
		bravo.Sync(group)
	})
}
