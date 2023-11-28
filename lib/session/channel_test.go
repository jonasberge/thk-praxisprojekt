package session

import (
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"projekt/core/lib/protocol"
	"testing"
)

type TestProtocol struct{}

// TODO: Type, TypeOf and NewMessage
//  could be generated automatically.

func (p TestProtocol) Id() protocol.Id {
	return protocol.Id(ChannelTypeInvalid)
}

func (p TestProtocol) TypeOf(message proto.Message) interface{} {
	switch message.(type) {
	case *Outer:
		return MessageTypeOuter
	default:
		panic(IllegalRootMessageType)
	}
}

func (p TestProtocol) RawTypeOf(message proto.Message) protocol.MessageType {
	return protocol.MessageType(p.TypeOf(message).(MessageType))
}

func (p TestProtocol) NewMessage(messageType protocol.MessageType) (proto.Message, error) {
	var message proto.Message
	switch MessageType(messageType) {
	case MessageTypeOuter:
		message = &Outer{}
	default:
		return nil, UnexpectedMessageType
	}
	return message, nil
}

var TP = TestProtocol{}

func TestChannelFactoryInitiator(t *testing.T) {
	f := NewChannelFactory(true)
	assert.Equal(t, ChannelId(1), f.CreateChannelHandle(TP).Id())
	assert.Equal(t, ChannelId(3), f.CreateChannelHandle(TP).Id())
}

func TestChannelFactoryResponder(t *testing.T) {
	f := NewChannelFactory(false)
	assert.Equal(t, ChannelId(2), f.CreateChannelHandle(TP).Id())
	assert.Equal(t, ChannelId(4), f.CreateChannelHandle(TP).Id())
}

func createChannel() ChannelHandle {
	return NewChannelFactory(true).CreateChannelHandle(TP)
}

func TestOpen(t *testing.T) {
	c := createChannel()
	m, err := proto.Marshal(c.OpenMessage())
	assert.Nil(t, err)
	cm, err := UnmarshalMessage(m)
	assert.Nil(t, err)
	assert.Equal(t, c.Id(), cm.ChannelId())
	assert.Equal(t, ChannelMessageTypeOpen, cm.Type())
	assert.True(t, cm.OpensChannel())
	assert.False(t, cm.AcceptsChannel())
	assert.False(t, cm.ClosesChannel())
	assert.False(t, cm.IsData())
}

func TestAccept(t *testing.T) {
	c := createChannel()
	m, err := proto.Marshal(c.AcceptMessage())
	assert.Nil(t, err)
	cm, err := UnmarshalMessage(m)
	assert.Nil(t, err)
	assert.Equal(t, c.Id(), cm.ChannelId())
	assert.Equal(t, ChannelMessageTypeAccept, cm.Type())
	assert.True(t, cm.AcceptsChannel())
	assert.False(t, cm.OpensChannel())
	assert.False(t, cm.ClosesChannel())
	assert.False(t, cm.IsData())
}

func TestClose(t *testing.T) {
	c := createChannel()
	m, err := proto.Marshal(c.CloseMessage())
	assert.Nil(t, err)
	cm, err := UnmarshalMessage(m)
	assert.Nil(t, err)
	assert.Equal(t, c.Id(), cm.ChannelId())
	assert.Equal(t, ChannelMessageTypeClose, cm.Type())
	assert.True(t, cm.ClosesChannel())
	assert.False(t, cm.OpensChannel())
	assert.False(t, cm.AcceptsChannel())
	assert.False(t, cm.IsData())
}

var inner = &Inner{A: 0xB}
var outer = &Outer{B: 0xA, Inner: inner}

func TestOuter(t *testing.T) {
	c := createChannel()
	m, err := c.MarshalMessage(outer)
	assert.Nil(t, err)
	cm, err := UnmarshalMessage(m)
	assert.Nil(t, err)
	assert.Equal(t, c.Id(), cm.ChannelId())
	assert.Equal(t, ChannelMessageTypeData, cm.Type())
	assert.False(t, cm.OpensChannel())
	assert.False(t, cm.ClosesChannel())
	assert.True(t, cm.IsData())
	tm, err := c.ReadMessage(cm)
	assert.Nil(t, err)
	assert.EqualValues(t, MessageTypeOuter, tm.Type)
	assert.True(t, proto.Equal(outer, tm.Content))
}

func TestInner(t *testing.T) {
	c := createChannel()
	assert.PanicsWithError(t, IllegalRootMessageType.Error(), func() {
		_, _ = c.MarshalMessage(inner)
	})
}
