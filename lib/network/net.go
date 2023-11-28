package network

import (
	"encoding/binary"
	"errors"
	"net"
)

type Net struct {
	net.IPNet
	Interface net.Interface
}

func (n *Net) IsUp() bool {
	return n.Interface.Flags&net.FlagUp != 0
}

func (n *Net) IsLoopback() bool {
	return n.Interface.Flags&net.FlagLoopback != 0
}

func (n *Net) IsBroadcast() bool {
	return n.Interface.Flags&net.FlagBroadcast != 0
}

func (n *Net) BroadcastIp() (ip net.IP, err error) {
	ip4 := n.IP.To4()
	if ip4 == nil {
		err = errors.New("not an IPv4 address")
		return
	}
	if len(ip4) != 4 {
		err = errors.New("ip is not 4 bytes long")
		return
	}
	ip = make(net.IP, len(ip4))
	addr := binary.BigEndian.Uint32(ip4)
	mask := binary.BigEndian.Uint32(n.Mask)
	binary.BigEndian.PutUint32(ip, addr|^mask)
	return
}
