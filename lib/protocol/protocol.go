package protocol

import (
	"errors"
	"google.golang.org/protobuf/proto"
)

var (
	NothingToWrite     = errors.New("there is nothing to write")
	UnexpectedMessage  = errors.New("read an unexpected message")
	IllegalMessageType = errors.New("illegal message type")
)

// Id identifies a protocol by a unique number.
// It must be unique across a set of protocols that might be spoken between the same peers.
type Id = int32

// MessageType represents the type of a message as a unique number.
type MessageType = int32

// Protocol defines the messages and types that are valid for a specific protocol.
// It is necessary for detecting and labeling messages with their corresponding predefined type
// and creating new messages when only their type is known.
type Protocol interface {
	// Id returns the unique Id of this protocol
	Id() Id

	// TypeOf returns the type of a Protobuf message.
	// The returned value may represent any implementation-specific value.
	TypeOf(proto.Message) interface{}

	// RawTypeOf returns the type of a Protobuf message as a plain MessageType.
	// The returned value is a numeric version
	// of the implementation-specific value that would be returned by TypeOf.
	// This is necessary for encoding the message type in a Protobuf message.
	RawTypeOf(proto.Message) MessageType

	// NewMessage creates a new Protobuf message which is represented by the given type.
	// This is necessary for instantiating a new message when only its type is given
	// that may have been encoded within another Protobuf message (hence the use of MessageType).
	NewMessage(MessageType) (proto.Message, error)
}
