package block

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	. "projekt/core/lib/util/buffer"
	"testing"
)

var WriteSize = Size(4)

func bufferContains(buffer *Buffer, block *Block) bool {
	offset := int(block.Index) * int(WriteSize)
	segment := buffer.Data[offset : offset+len(block.Data)]
	return bytes.Equal(segment, block.Data)
}

func createBlockWriter() (*Buffer, *Writer) {
	size := len(Data)
	buffer := NewBuffer(make([]byte, size))
	writer := NewWriter(buffer, uint64(size), WriteSize)
	return buffer, writer
}

func checkBlockExpect(t *testing.T, w *Writer, block *Block, err error) {
	e := w.checkBlock(block)
	if err != nil {
		assert.ErrorIs(t, e, err)
	} else {
		assert.Nil(t, e)
	}
}

func TestBlockWriter_checkBlock(t *testing.T) {
	_, w := createBlockWriter()
	checkBlockExpect(t, w, newBlock(2, FirstBlock.Data), WriterBoundsExceeded)
	checkBlockExpect(t, w, newBlock(FirstBlock.Index, Data[0:3]), BadBlockData)
	checkBlockExpect(t, w, newBlock(FirstBlock.Index, Data[0:5]), BadBlockData)
	err := w.WriteBlock(FirstBlock)
	assert.Nil(t, err)
	checkBlockExpect(t, w, FirstBlock, AlreadyWritten)
}

func TestBlockWriter_WriteBlock(t *testing.T) {
	b, w := createBlockWriter()
	err := w.WriteBlock(FirstBlock)
	assert.Nil(t, err)
	assert.True(t, bufferContains(b, FirstBlock))
	assert.False(t, w.IsDone())
	err = w.WriteBlock(LastBlock)
	assert.Nil(t, err)
	assert.True(t, bufferContains(b, LastBlock))
	assert.True(t, w.IsDone())
	err = w.WriteBlock(FirstBlock)
	assert.ErrorIs(t, err, AlreadyWritten)
	assert.EqualValues(t, Data, b.Data)
}
