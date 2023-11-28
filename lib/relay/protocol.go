package relay

import (
	"google.golang.org/protobuf/proto"
	"projekt/core/lib/protocol"
)

type Protocol struct{}

func (p Protocol) Id() protocol.Id {
	return protocol.Id(1)
}

func (p Protocol) TypeOf(message proto.Message) interface{} {
	switch message.(type) {
	case *Connected:
		return MessageTypeConnected
	case *AlreadyConnected:
		return MessageTypeAlreadyConnected
	case *RequestSession:
		return MessageTypeRequestSession
	case *SessionFailed:
		return MessageTypeSessionFailed
	case *SessionCreated:
		return MessageTypeSessionCreated
	case *SessionInvitation:
		return MessageTypeSessionInvitation
	case *AcceptSession:
		return MessageTypeAcceptSession
	case *SessionAccepted:
		return MessageTypeSessionAccepted
	case *SessionEstablished:
		return MessageTypeSessionEstablished
	default:
		panic(protocol.IllegalMessageType)
	}
}

func (p Protocol) RawTypeOf(message proto.Message) protocol.MessageType {
	return protocol.MessageType(p.TypeOf(message).(MessageType))
}

func (p Protocol) NewMessage(messageType protocol.MessageType) (m proto.Message, err error) {
	switch MessageType(messageType) {
	case MessageTypeConnected:
		m = &Connected{}
	case MessageTypeAlreadyConnected:
		m = &AlreadyConnected{}
	case MessageTypeRequestSession:
		m = &RequestSession{}
	case MessageTypeSessionFailed:
		m = &SessionFailed{}
	case MessageTypeSessionCreated:
		m = &SessionCreated{}
	case MessageTypeSessionInvitation:
		m = &SessionInvitation{}
	case MessageTypeAcceptSession:
		m = &AcceptSession{}
	case MessageTypeSessionAccepted:
		m = &SessionAccepted{}
	case MessageTypeSessionEstablished:
		m = &SessionEstablished{}
	default:
		err = protocol.IllegalMessageType
	}
	return
}

func (p Protocol) WrapMessage(message proto.Message) (wrapped []byte, err error) {
	// TODO Make this part of the Protocol interface.
	//  Or even better: The wrapping algorithm is the same for every protocol,
	//  so factor it out into a reusable function.
	content, err := proto.Marshal(message)
	if err != nil {
		return
	}
	wrapped, err = proto.Marshal(&Message{
		Type:    MessageType(p.RawTypeOf(message)),
		Content: content,
	})
	return
}

func (p Protocol) UnwrapMessage(wrapped []byte) (message proto.Message, err error) {
	root := &Message{}
	err = proto.Unmarshal(wrapped, root)
	if err != nil {
		return
	}
	message, err = p.NewMessage(protocol.MessageType(root.Type))
	if err != nil {
		return
	}
	err = proto.Unmarshal(root.Content, message)
	if err != nil {
		message = nil
	}
	return
}
