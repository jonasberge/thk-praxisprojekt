package beacon

import (
	"net"
	"projekt/core/lib/network"
)

type Beacon interface {
	Listen() error
	Write(data []byte, dst network.Net) (err error)
	Read() (data []byte, addr net.Addr, err error)
	Close() error
}

func NewBeacon() Beacon {
	// TODO for now only a broadcast beacon, multicast can wait
	panic("not implemented")
	return nil // &compound{}
}
