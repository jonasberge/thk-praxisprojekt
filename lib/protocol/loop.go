package protocol

import (
	"google.golang.org/protobuf/proto"
	"log"
)

type ReadWriter interface {
	Write(message proto.Message) (err error)
	Read() (message proto.Message, err error)
}

// Loop represents a read-write loop for a protocol Client.
type Loop struct {
	client *Client
	conn   ReadWriter
	info   *log.Logger
}

func NewLoop(client *Client, conn ReadWriter) *Loop {
	return &Loop{
		client: client,
		conn:   conn,
		info:   log.New(log.Writer(), "PROTOCOL ", log.Flags()|log.Lmsgprefix),
	}
}

// Run runs a blocking loop that reads and writes messages for a protocol Client
// until either the protocol execution is done or an error occurs.
func (l *Loop) Run() (err error) {
	in := make(chan proto.Message)
	errs := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		for {
			// TODO this "conn" is a channel and it MUST return an error once it is closed.
			message, e := l.conn.Read()
			if e != nil {
				errs <- e
				return
			}
			l.info.Println("<-", message.ProtoReflect().Descriptor().Name())
			select {
			case in <- message:
			case <-done:
				return
			}
		}
	}()
	for {
		var message proto.Message
		if l.client.IsDone() {
			close(done)
			return nil
		}
		if l.client.MustWrite() {
			if err = l.write(); err != nil {
				return
			}
			continue
		}
		if l.client.CanWrite() {
			select {
			case message = <-in:
				err = l.client.ReadMessage(message)
				if err != nil {
					return
				}
			case err = <-errs:
				return
			default:
				if err = l.write(); err != nil {
					return
				}
			}
			continue
		}
		select {
		case message = <-in:
			err = l.client.ReadMessage(message)
			if err != nil {
				return
			}
		case err = <-errs:
			return
		}
	}
}

func (l *Loop) write() (err error) {
	message, err := l.client.WriteMessage()
	if err != nil {
		return
	}
	err = l.conn.Write(message)
	l.info.Println("->", message.ProtoReflect().Descriptor().Name())
	return
}
