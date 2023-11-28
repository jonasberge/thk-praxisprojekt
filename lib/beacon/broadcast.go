package beacon

import (
	"context"
	"errors"
	"net"
	"projekt/core/lib/network"
	"time"
)

var writeTimeout = time.Second

type outPacket struct {
	data []byte
	ip   net.IP
	err  chan<- error
}

type inPacket struct {
	data []byte
	addr net.Addr
}

type broadcast struct {
	port   int
	out    chan outPacket
	ins    chan chan inPacket
	ctx    context.Context
	cancel context.CancelFunc
}

func NewBroadcast(ctx context.Context, port int) Beacon {
	ctx, cancel := context.WithCancel(ctx)
	return &broadcast{
		port:   port,
		out:    make(chan outPacket),
		ins:    make(chan chan inPacket),
		ctx:    ctx,
		cancel: cancel,
	}
}

func (b *broadcast) Listen() (err error) {
	// Let the reader bind to the specified port first
	// before the writer can accidentally bind to it by picking a random one.
	// We also won't unnecessarily open the writer if the port is already in use.
	rc, err := b.listenReader()
	if err != nil {
		return
	}
	wc, err := b.listenWriter()
	if err != nil {
		_ = rc.Close() // make sure to close this
		return
	}
	go func() {
		<-b.ctx.Done()
		_ = concurrent(rc.Close, wc.Close)
	}()
	return
}

func (b *broadcast) listenWriter() (conn *net.UDPConn, err error) {
	conn, err = net.ListenUDP("udp4", nil)
	if err != nil {
		return
	}
	go b.handleWrites(conn)
	return
}

func (b *broadcast) listenReader() (conn *net.UDPConn, err error) {
	addr := &net.UDPAddr{Port: b.port}
	conn, err = net.ListenUDP("udp4", addr)
	if err != nil {
		return
	}
	go b.handleReads(conn)
	return
}

func (b *broadcast) handleWrites(conn *net.UDPConn) {
	for {
		var packet outPacket
		select {
		case packet = <-b.out:
		case <-b.ctx.Done():
			return
		}
		go func() {
			addr := &net.UDPAddr{IP: packet.ip, Port: b.port}
			err := conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err != nil {
				packet.err <- err
				return
			}
			n, err := conn.WriteToUDP(packet.data, addr)
			if err != nil {
				packet.err <- err
				return
			}
			if n != len(packet.data) {
				panic("unexpected nil error")
			}
			err = conn.SetWriteDeadline(time.Time{})
			if err != nil {
				packet.err <- err
				return
			}
		}()
	}
}

func (b *broadcast) handleReads(conn *net.UDPConn) {
	buf := make([]byte, 1<<16)
	ch := make(chan inPacket)
	go func() {
		for {
			n, addr, err := conn.ReadFrom(buf)
			// TODO we might be missing packets while ReadFrom is not called and blocking.
			if err != nil {
				close(ch)
				return
			}
			data := make([]byte, n)
			copy(data, buf)
			ch <- inPacket{data, addr}
		}
	}()
	ins := make([]chan inPacket, 0, 4)
	for {
		select {
		case p := <-ch:
			for _, in := range ins {
				select {
				case in <- p:
				case <-b.ctx.Done():
					return
				}
			}
			ins = ins[:0]
		case in := <-b.ins:
			ins = append(ins, in)
		case <-b.ctx.Done():
			return
		}
	}
}

func (b *broadcast) Write(data []byte, dst network.Net) (err error) {
	if !dst.IsBroadcast() {
		return errors.New("destination does not support broadcasting")
	}
	ip, err := dst.BroadcastIp()
	if err != nil {
		return
	}
	errCh := make(chan error)
	select {
	case b.out <- outPacket{data, ip, errCh}:
	case err = <-errCh:
		// FIXME we might receive an error of another Write() call here
	case <-b.ctx.Done():
		err = net.ErrClosed
	}
	return
}

func (b *broadcast) Read() (data []byte, addr net.Addr, err error) {
	in := make(chan inPacket)
	b.ins <- in
	select {
	case p := <-in:
		addr = p.addr
		data = make([]byte, len(p.data))
		copy(data, p.data)
	case <-b.ctx.Done():
		err = net.ErrClosed
	}
	return
}

func (b *broadcast) Close() error {
	b.cancel()
	return nil
}
