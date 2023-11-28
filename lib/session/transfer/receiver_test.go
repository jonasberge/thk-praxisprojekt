package transfer

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"math/rand"
	"projekt/core/lib/protocol"
	"projekt/core/lib/session/transfer/block"
	. "projekt/core/lib/util/buffer"
	"testing"
)

const (
	DataSize      = 16
	DataBlockSize = 4
)

func createRandomData(r *rand.Rand) (data []byte) {
	data = make([]byte, DataSize)
	r.Read(data)
	return
}

func createBlockReader2(data []byte) *block.Reader {
	reader := NewBuffer(data)
	return block.NewBlockReader(reader, uint64(len(data)), DataBlockSize)
}

func createRandomSender(r *rand.Rand) (in []byte, sender *Sender) {
	in = createRandomData(r)
	sender = NewSender(MessageMetadata, createBlockReader2(in))
	return
}

func createReceiver() (out []byte, receiver *Receiver) {
	out = make([]byte, DataSize)
	receiver = NewReceiver(func(metadata *Metadata, size uint64) (block.WriteSeekCloser, error) {
		return NewBuffer(out), nil
	})
	return
}

func createRandomSenderReceiver(r *rand.Rand) (in []byte, out []byte, sender *Sender, receiver *Receiver) {
	in, sender = createRandomSender(r)
	out, receiver = createReceiver()
	return
}

func receiverReadMessage(t *testing.T, s *Receiver, message proto.Message) {
	err := s.ReadMessage(message)
	assert.Nil(t, err)
}

func receiverReadMessageExpectError(t *testing.T, s *Receiver, message proto.Message, expectedError error) {
	err := s.ReadMessage(message)
	assert.ErrorIs(t, err, expectedError)
}

func receiverWriteExpectMessage(t *testing.T, s *Receiver, expectedMessage proto.Message) {
	assert.True(t, s.CanWrite())
	m, err := s.WriteMessage()
	assert.Nil(t, err)
	assert.True(t, proto.Equal(expectedMessage, m))
	// We are only done after a DataReceived from the receiver
	// which needs to be passed to ReadMessage.
	assert.False(t, s.IsDone())
}

func receiverWriteExpectError(t *testing.T, s *Receiver, expectedError error) {
	m, err := s.WriteMessage()
	assert.ErrorIs(t, err, expectedError)
	assert.Nil(t, m)
}

func receiverExpectNothingToWrite(t *testing.T, s *Receiver) {
	assert.False(t, s.CanWrite())
	receiverWriteExpectError(t, s, protocol.NothingToWrite)
}

func Test_Sender_Receiver(t *testing.T) {
	rnd := rand.New(rand.NewSource(0))
	i, o, s, r := createRandomSenderReceiver(rnd)
	fmt.Println(i)
	fmt.Println(o)

	k := 0

	for !s.IsDone() {
		k += 1
		fmt.Printf("[%v]\n", k)

		senderMessage, err := s.WriteMessage()
		assert.Nil(t, err)
		fmt.Println("->", P.TypeOf(senderMessage), senderMessage)
		err = r.ReadMessage(senderMessage)
		assert.Nil(t, err)
		for r.CanWrite() {
			receiverMessage, err := r.WriteMessage()
			assert.Nil(t, err)
			fmt.Println("<-", P.TypeOf(receiverMessage), receiverMessage)
			err = s.ReadMessage(receiverMessage)
			assert.Nil(t, err)
		}
	}

	assert.True(t, r.IsDone())
	assert.EqualValues(t, i, o)
}
