package session

import (
	"errors"
	"google.golang.org/protobuf/proto"
	"log"
	"math"
	"projekt/core/lib/protocol"
	"projekt/core/lib/secure"
	"sync"
)

const (
	firstChannelId     = 1
	channelIdIncrement = 2
)

var (
	NoMessageContent       = errors.New("no message content")
	UnexpectedMessageType  = errors.New("unexpected message type")
	ExpectedDataMessage    = errors.New("expected data message type")
	IllegalRootMessageType = errors.New("illegal root message type")
)

var PayloadTooLarge = errors.New("the payload is too large")

// TODO This is not possible. Protobuf will panic. Is this a bug?
//var PayloadMaxSize = 0
//func init() {
//	PayloadMaxSize = ComputePayloadMaxSize()
//}

var payloadMaxSize = 0
var payloadMaxSizeInitOnce = sync.Once{}

func initPayloadMaxSize() {
	payloadMaxSize = ComputePayloadMaxSize()
}

// ComputePayloadMaxSize computes the maximum size of the payload of a Data message.
func ComputePayloadMaxSize() int {
	const PayloadMaxSize = secure.PayloadMaxSize
	payload := make([]byte, PayloadMaxSize)
	message := &Message{
		Content: &Message_Data{
			Data: &Data{
				// Use the maximum number because Protobuf uses varint encoding.
				ChannelId:   math.MaxUint64,
				MessageType: math.MaxInt32,
				Payload:     payload,
			},
		},
	}
	encoded, err := proto.Marshal(message)
	if err != nil {
		log.Printf("BUG: failed to marshal maximum payload message: %v", err)
		// fall back to something small enough in case there is an error.
		return PayloadMaxSize - 256
	}
	tooMuch := len(encoded) - PayloadMaxSize
	return PayloadMaxSize - tooMuch
}

type ChannelId = uint64

// ChannelFactory is used per connection to create new unique channels.
type ChannelFactory struct {
	nextId ChannelId
	mutex  sync.Mutex
}

func NewChannelFactory(initiator bool) *ChannelFactory {
	factory := &ChannelFactory{
		nextId: firstChannelId,
		mutex:  sync.Mutex{},
	}
	if !initiator {
		factory.nextId += 1
	}
	return factory
}

func (f *ChannelFactory) CreateChannelHandle(protocol protocol.Protocol) ChannelHandle {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	id := f.nextId
	f.nextId += channelIdIncrement
	// TODO: handle overflow (not going to happen practically though)
	return NewChannelHandle(protocol, id)
}

type ChannelHandle interface {
	Protocol() protocol.Protocol
	Type() protocol.Id
	Id() ChannelId
	PayloadMaxSize() int
	OpenMessage() *Message
	AcceptMessage() *Message
	CloseMessage() *Message
	WrapMessage(message proto.Message) (*Message, error)
	MarshalMessage(message proto.Message) ([]byte, error)
	ReadMessage(message ChannelMessage) (protocol.Message, error)
}

type channelHandle struct {
	protocol protocol.Protocol
	id       ChannelId
}

func NewChannelHandle(protocol protocol.Protocol, channelId ChannelId) ChannelHandle {
	payloadMaxSizeInitOnce.Do(initPayloadMaxSize)
	return &channelHandle{
		protocol: protocol,
		id:       channelId,
	}
}

func (c *channelHandle) Protocol() protocol.Protocol {
	return c.protocol
}

func (c *channelHandle) Type() protocol.Id {
	return c.protocol.Id()
}

func (c *channelHandle) Id() ChannelId {
	return c.id
}

func (c *channelHandle) PayloadMaxSize() int {
	return payloadMaxSize
}

func (c *channelHandle) OpenMessage() *Message {
	return &Message{
		Content: &Message_Open{
			Open: &Open{
				ChannelId:   c.id,
				ChannelType: ChannelType(c.Type()), // FIXME
			},
		},
	}
}

func (c *channelHandle) AcceptMessage() *Message {
	return &Message{
		Content: &Message_Accept{
			Accept: &Accept{
				ChannelId: c.id,
			},
		},
	}
}

func (c *channelHandle) CloseMessage() *Message {
	return &Message{
		Content: &Message_Close{
			Close: &Close{
				ChannelId: c.id,
			},
		},
	}
}

func (c *channelHandle) WrapMessage(message proto.Message) (*Message, error) {
	payload, err := proto.Marshal(message)
	if err != nil {
		return nil, err
	}
	if len(payload) > c.PayloadMaxSize() {
		return nil, PayloadTooLarge
	}
	return &Message{
		Content: &Message_Data{
			Data: &Data{
				ChannelId:   c.id,
				MessageType: c.protocol.RawTypeOf(message),
				Payload:     payload,
			},
		},
	}, nil
}

func (c *channelHandle) MarshalMessage(message proto.Message) ([]byte, error) {
	result, err := c.WrapMessage(message)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(result)
}

func (c *channelHandle) ReadMessage(message ChannelMessage) (protocol.Message, error) {
	if !message.IsData() {
		return protocol.EmptyTypedMessage, ExpectedDataMessage
	}
	data, ok := message.Content().(*Data)
	if !ok {
		panic("message should have been of type data")
	}
	result, err := c.protocol.NewMessage(data.MessageType)
	if err != nil {
		return protocol.EmptyTypedMessage, err
	}
	err = proto.Unmarshal(data.Payload, result)
	return protocol.Message{
		Content: result,
		Type:    data.MessageType,
	}, nil
}

type ChannelMessageType = uint16

const (
	ChannelMessageTypeOpen ChannelMessageType = iota + 1
	ChannelMessageTypeAccept
	ChannelMessageTypeClose
	ChannelMessageTypeData
)

type ChannelMessage interface {
	ChannelId() ChannelId
	Type() ChannelMessageType
	OpensChannel() bool
	AcceptsChannel() bool
	ClosesChannel() bool
	IsData() bool
	Content() proto.Message
}

type channelMessage struct {
	content     proto.Message
	channelId   ChannelId
	messageType ChannelMessageType
}

func (m *channelMessage) ChannelId() ChannelId {
	return m.channelId
}

func (m *channelMessage) Type() ChannelMessageType {
	return m.messageType
}

func (m *channelMessage) OpensChannel() bool {
	return m.Type() == ChannelMessageTypeOpen
}

func (m *channelMessage) AcceptsChannel() bool {
	return m.Type() == ChannelMessageTypeAccept
}

func (m *channelMessage) ClosesChannel() bool {
	return m.Type() == ChannelMessageTypeClose
}

func (m *channelMessage) IsData() bool {
	return m.Type() == ChannelMessageTypeData
}

func (m *channelMessage) Content() proto.Message {
	return m.content
}

func UnwrapMessage(message *Message) (ChannelMessage, error) {
	switch content := message.Content.(type) {
	case *Message_Open:
		return &channelMessage{content.Open, content.Open.ChannelId, ChannelMessageTypeOpen}, nil
	case *Message_Accept:
		return &channelMessage{content.Accept, content.Accept.ChannelId, ChannelMessageTypeAccept}, nil
	case *Message_Close:
		return &channelMessage{content.Close, content.Close.ChannelId, ChannelMessageTypeClose}, nil
	case *Message_Data:
		return &channelMessage{content.Data, content.Data.ChannelId, ChannelMessageTypeData}, nil
	case nil:
		return nil, NoMessageContent
	default:
		return nil, UnexpectedMessageType
	}
}

func UnmarshalMessage(bytes []byte) (ChannelMessage, error) {
	message := Message{}
	err := proto.Unmarshal(bytes, &message)
	if err != nil {
		return nil, err
	}
	return UnwrapMessage(&message)
}
