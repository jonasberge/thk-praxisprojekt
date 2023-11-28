//+build container

package beacon

import (
	"context"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/nettest"
	"log"
	"net"
	"projekt/core/lib/network"
	"projekt/core/lib/util/container"
	"testing"
	"time"
)

// Run tests with:
// $ sudo make container_start  # run this if the containers were not started yet
// $ sudo go run test.go beacon

const (
	port = 23520
	data = "something useful"
)

var writer = container.New("alpha")
var reader = container.New("bravo")

func getNet(t *testing.T) network.Net {
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

func createBroadcast(t *testing.T) (Beacon, func()) {
	ctx := context.Background()
	bc := NewBroadcast(ctx, port)
	err := bc.Listen()
	assert.Nil(t, err)
	return bc, func() {
		// Let any write and read operations finish gracefully.
		time.Sleep(10 * time.Millisecond)
		bc.Close()
	}
}

func TestBroadcast_ReadWrite(t *testing.T) {
	group := container.NewGroup(writer, reader)
	writer.Run(func() {
		bc, done := createBroadcast(t)
		defer done()
		writer.Sync(group)
		err := bc.Write([]byte(data), getNet(t))
		assert.Nil(t, err)
	})
	reader.RunAsync(func() {
		bc, done := createBroadcast(t)
		go func() {
			defer reader.Done()
			defer done()
			bytes, addr, err := bc.Read()
			assert.Nil(t, err)
			assert.EqualValues(t, data, string(bytes))
			assert.True(t, writer.HasIP(addr.(*net.UDPAddr).IP))
		}()
		reader.Sync(group)
	})
}
