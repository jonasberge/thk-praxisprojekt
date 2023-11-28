package transfer

import (
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"projekt/core/lib/protocol"
	"projekt/core/lib/session/transfer/block"
	"projekt/core/lib/util/buffer"
	"testing"
)

var P = Protocol{}

var Data = []byte("01234") // 0123 4
var Size = block.Size(4)
var FirstBlock = newBlock(0, Data[0:4])
var LastBlock = newBlock(1, Data[4:])

var (
	MessageMetadata = &Metadata{Filename: "whatever.idk"}
	MessageHeader   = &Header{
		Metadata:  MessageMetadata,
		TotalSize: uint64(len(Data)),
		BlockSize: uint64(Size),
	}
	MessageAcknowledge        = &Acknowledge{}
	MessageFirstBlockReceived = &BlockReceived{Index: FirstBlock.Index}
	MessageLastBlockReceived  = &BlockReceived{Index: LastBlock.Index}
	MessageRedoLastBlock      = &RedoBlock{Index: LastBlock.Index}
	MessageDataSent           = &DataSent{} // TODO: TotalChecksum
	MessageDataReceived       = &DataReceived{}
)

func newBlock(index uint64, data []byte) *Block {
	return &Block{
		Index:    index,
		Data:     data,
		Checksum: block.ComputeChecksum(index, data),
	}
}

func createBlockReader() *block.Reader {
	reader := buffer.NewBuffer(Data)
	return block.NewBlockReader(reader, uint64(len(Data)), Size)
}

func createSender() *Sender {
	return NewSender(MessageMetadata, createBlockReader())
}

func senderReadMessage(t *testing.T, s *Sender, message proto.Message) {
	err := s.ReadMessage(message)
	assert.Nil(t, err)
}

func senderReadMessageExpectError(t *testing.T, s *Sender, message proto.Message, expectedError error) {
	err := s.ReadMessage(message)
	assert.ErrorIs(t, err, expectedError)
}

func senderWriteExpectMessage(t *testing.T, s *Sender, expectedMessage proto.Message) {
	assert.True(t, s.CanWrite())
	m, err := s.WriteMessage()
	assert.Nil(t, err)
	assert.True(t, proto.Equal(expectedMessage, m))
	// We are only done after a DataReceived from the receiver
	// which needs to be passed to ReadMessage.
	assert.False(t, s.IsDone())
}

func senderWriteExpectError(t *testing.T, s *Sender, expectedError error) {
	m, err := s.WriteMessage()
	assert.ErrorIs(t, err, expectedError)
	assert.Nil(t, m)
}

func senderExpectNothingToWrite(t *testing.T, s *Sender) {
	assert.False(t, s.CanWrite())
	senderWriteExpectError(t, s, protocol.NothingToWrite)
}

func TestSender(t *testing.T) {
	s := createSender()
	senderWriteExpectMessage(t, s, MessageHeader)
	senderWriteExpectMessage(t, s, FirstBlock)
	senderReadMessageExpectError(t, s, MessageFirstBlockReceived, protocol.UnexpectedMessage)
	senderReadMessage(t, s, MessageAcknowledge)
	senderReadMessage(t, s, MessageFirstBlockReceived)
	senderWriteExpectMessage(t, s, LastBlock)
	senderWriteExpectMessage(t, s, MessageDataSent)
	senderExpectNothingToWrite(t, s)
	senderReadMessage(t, s, MessageRedoLastBlock)
	senderWriteExpectMessage(t, s, LastBlock)
	senderWriteExpectMessage(t, s, MessageDataSent)
	senderExpectNothingToWrite(t, s)
	senderReadMessage(t, s, MessageLastBlockReceived)
	senderExpectNothingToWrite(t, s)
	senderReadMessage(t, s, MessageDataReceived)
	assert.True(t, s.IsDone())
	senderExpectNothingToWrite(t, s)
	senderReadMessageExpectError(t, s, MessageDataReceived, protocol.UnexpectedMessage)
}
