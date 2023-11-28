package transfer

import (
	"google.golang.org/protobuf/proto"
	"projekt/core/lib/protocol"
	"projekt/core/lib/session"
)

type Protocol struct{}

func (p Protocol) Id() protocol.Id {
	return protocol.Id(session.ChannelTypeTransfer)
}

func (p Protocol) TypeOf(message proto.Message) interface{} {
	switch message.(type) {
	case *Header:
		return MessageTypeHeader
	case *Acknowledge:
		return MessageTypeAcknowledge
	case *Block:
		return MessageTypeBlock
	case *BlockReceived:
		return MessageTypeBlockReceived
	case *RedoBlock:
		return MessageTypeRedoBlock
	case *DataSent:
		return MessageTypeDataSent
	case *DataReceived:
		return MessageTypeDataReceived
	case *Abort:
		return MessageTypeAbort
	default:
		panic(session.IllegalRootMessageType)
	}
}

func (p Protocol) RawTypeOf(message proto.Message) protocol.MessageType {
	return protocol.MessageType(p.TypeOf(message).(MessageType))
}

func (p Protocol) NewMessage(messageType protocol.MessageType) (m proto.Message, err error) {
	switch MessageType(messageType) {
	case MessageTypeHeader:
		m = &Header{}
	case MessageTypeAcknowledge:
		m = &Acknowledge{}
	case MessageTypeBlock:
		m = &Block{}
	case MessageTypeBlockReceived:
		m = &BlockReceived{}
	case MessageTypeRedoBlock:
		m = &RedoBlock{}
	case MessageTypeDataSent:
		m = &DataSent{}
	case MessageTypeDataReceived:
		m = &DataReceived{}
	case MessageTypeAbort:
		m = &Abort{}
	default:
		err = session.UnexpectedMessageType
	}
	return
}
