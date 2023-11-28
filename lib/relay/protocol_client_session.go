package relay

import (
	"errors"
	"google.golang.org/protobuf/proto"
	"net"
	"projekt/core/lib/packet"
	"projekt/core/lib/protocol"
)

var (
	ErrSessionAlreadyConnected = errors.New("already connected to this session")
	ErrSessionKeyInvalid       = errors.New("the session key is invalid")
)

type SessionClientProtocol struct {
	conn net.Conn
}

func NewSessionClientProtocol(sessionConn net.Conn) *SessionClientProtocol {
	return &SessionClientProtocol{
		conn: sessionConn,
	}
}

func (c *SessionClientProtocol) Authenticate(key []byte) (conn net.Conn, err error) {
	data, err := proto.Marshal(&AuthenticateSession{Key: key})
	if err != nil {
		return
	}
	_, err = packet.New(data).WriteTo(c.conn)
	if err != nil {
		return
	}
	result, err := packet.DecodeFrom(c.conn)
	if err != nil {
		return
	}
	message := &SessionResponse{}
	err = proto.Unmarshal(result, message)
	if err != nil {
		return
	}
	if message.State != SessionStateConnected {
		err = sessionStateToError(message.State)
		return
	}
	conn = c.conn
	return
}

func sessionStateToError(state SessionState) error {
	switch state {
	case SessionStateAlreadyConnected:
		return ErrSessionAlreadyConnected
	case SessionStateInvalidKey:
		return ErrSessionKeyInvalid
	case SessionStateConnected:
		panic("connected is not an error")
	default:
		return protocol.UnexpectedMessage
	}
}
