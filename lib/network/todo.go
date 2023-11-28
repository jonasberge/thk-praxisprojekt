package network

import (
	"context"
	"net"
)

// todoTracker is a Tracker that does not yet track future network changes.
type todoTracker struct{}

func (t *todoTracker) Listen(ctx context.Context) (present []Net, future <-chan *Net, err error) {
	// TODO track changes and write them to the future channel
	//  also close the future channel once the context is done
	ins, err := net.Interfaces()
	if err != nil {
		return
	}
	present = make([]Net, 0, 8)
	for _, in := range ins {
		if in.Flags&net.FlagUp == 0 {
			// ignore interfaces that are down
			continue
		}
		var inAddrs []net.Addr
		inAddrs, err = in.Addrs()
		if err != nil {
			return
		}
		for _, inAddr := range inAddrs {
			addr, ok := inAddr.(*net.IPNet)
			if !ok {
				continue
			}
			n := Net{
				IPNet:     *addr,
				Interface: in,
			}
			present = append(present, n)
		}
	}
	return
}
