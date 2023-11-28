package network

import "context"

// Tracker tracks which networks this device is currently connected to.
// One can request to get updates on newly connected networks.
// While there are open requests, the tracker polls for network changes.
// The polling rate can be configured in the constructor of the type.
type Tracker interface {
	Listen(ctx context.Context) (present []Net, future <-chan *Net, err error)
}

func NewTracker() Tracker {
	return &todoTracker{}
}
