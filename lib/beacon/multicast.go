package beacon

import "net"

type multicast struct {
}

func NewMulticast(group string) Beacon {
	// specify the group which needs to be joined for multicasts
	// we MUST be listening for new interfaces while there is someone who'd like to read.
	// join these interfaces on demand (maybe use a tracker internally?)
	return nil // &multicast{}
}

func (m *multicast) Listen() error {
	return nil
}

func (m *multicast) Write(data []byte, in net.Interface) {

}

func (m *multicast) Read() (data []byte, addr net.Addr) {
	return nil, nil
}

func (m *multicast) Close() error {
	return nil
}
