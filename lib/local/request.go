package local

import (
	"context"
	"encoding/binary"
	"net"
)

type RequestId uint64

func (id RequestId) Bytes() []byte {
	bytes := make([]byte, binary.MaxVarintLen64)
	binary.BigEndian.PutUint64(bytes, uint64(id))
	return bytes
}

type Request struct {
	Id               RequestId
	HandshakeMessage []byte
}

type RequestHandler interface {
	Handle(request Request, conn net.Conn) (ok bool)
}

type RequestListener interface {
	Accept() (request Request, conn net.Conn, err error)
}

func NewRequestListener(ctx context.Context) (RequestListener, RequestHandler) {
	l := &requestProcessor{
		Processor: NewProcessor(ctx),
	}
	return l, l
}

type requestProcessor struct {
	*Processor
}

func (p *requestProcessor) Handle(request Request, conn net.Conn) (ok bool) {
	return p.Processor.Handle(request, conn)
}

func (p *requestProcessor) Error(err error) {
	p.Processor.Error(err)
}

func (p *requestProcessor) Accept() (request Request, conn net.Conn, err error) {
	message, conn, err := p.Processor.Accept()
	if err == nil {
		request = message.(Request)
	}
	return
}
