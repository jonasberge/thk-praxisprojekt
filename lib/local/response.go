package local

import (
	"context"
	"net"
)

type Response struct {
	RequestId        RequestId
	HandshakeMessage []byte
}

type ResponseListener interface {
	Accept() (response Response, conn net.Conn, err error)
}

type ResponseHandler interface {
	Handle(response Response, conn net.Conn) (ok bool)
}

func NewResponseListener(ctx context.Context) (ResponseListener, ResponseHandler) {
	l := &responseProcessor{
		Processor: NewProcessor(ctx),
	}
	return l, l
}

type responseProcessor struct {
	*Processor
}

func (p *responseProcessor) Handle(response Response, conn net.Conn) (ok bool) {
	return p.Processor.Handle(response, conn)
}

func (p *responseProcessor) Error(err error) {
	p.Processor.Error(err)
}

func (p *responseProcessor) Accept() (response Response, conn net.Conn, err error) {
	message, conn, err := p.Processor.Accept()
	if err == nil {
		response = message.(Response)
	}
	return
}
