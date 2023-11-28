package protocol

import "google.golang.org/protobuf/proto"

type Message struct {
	Content proto.Message
	Type    interface{}
}

var EmptyTypedMessage = Message{}

func (m *Message) Is(messageType interface{}) bool {
	return m.Type == messageType
}

func (m *Message) IsNot(messageType interface{}) bool {
	return !m.Is(messageType)
}
