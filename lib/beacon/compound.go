package beacon

import (
	"net"
)

// compound combines broadcast and multicast.
type compound struct {
	broadcast
	multicast
}

func (c *compound) Listen() error {
	return concurrent(c.broadcast.Listen, c.multicast.Listen)
}

func (c *compound) Write(data []byte) {
}

func (c *compound) Read() (data []byte, addr net.Addr) {
	return nil, nil
}

func (c *compound) Close() error {
	return nil // return concurrent(c.broadcast.Close, c.multicast.Close)
}
