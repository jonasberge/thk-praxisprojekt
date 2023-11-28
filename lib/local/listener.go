package local

import (
	"context"
	"net"
)

// Listener listens for any message and its respective net.Conn.
type Listener interface {
	// Accept blocks until a message is received.
	// The associated net.Conn is a connection with the device which sent the message.
	// This method blocks until there is a message or the operation is cancelled.
	// A returned error indicates that this listener is in failed state
	// and cannot be used to accept new messages.
	// This method should only be called by one goroutine.
	Accept() (message interface{}, conn net.Conn, err error)
}

type Handler interface {
	// Handle hands the message and net.Conn to the coupled Listener.
	// This method may block until the message is accepted with Listener.Accept.
	// The returned boolean value indicates if the message could be handled or not.
	// The caller should close and discard the connection if the return value is false.
	Handle(message interface{}, conn net.Conn) (ok bool)

	// Error reports an error to the coupled Listener.
	Error(err error)
}

// NewListener creates a Listener and couples it with a Handler.
// Methods that are called on either of these instances directly have impact on the other.
// The listener is closed once the context is done.
func NewListener(ctx context.Context) (Listener, Handler) {
	l := NewProcessor(ctx)
	return l, l
}

func NewProcessor(ctx context.Context) *Processor {
	return &Processor{
		ctx: ctx,
		out: make(chan messageConn),
		err: make(chan error),
	}
}

type messageConn struct {
	message interface{}
	conn    net.Conn
}

type Processor struct {
	ctx context.Context
	out chan messageConn
	err chan error
}

func (p *Processor) Handle(message interface{}, conn net.Conn) (ok bool) {
	select {
	case p.out <- messageConn{message, conn}:
		return true
	case <-p.ctx.Done():
		return false
	}
}

func (p *Processor) Error(err error) {
	select {
	case p.err <- err:
	case <-p.ctx.Done():
	}
}

func (p *Processor) Accept() (message interface{}, conn net.Conn, err error) {
	select {
	case out := <-p.out:
		message = out.message
		conn = out.conn
	case err = <-p.err:
	case <-p.ctx.Done():
		err = p.ctx.Err()
	}
	return
}
