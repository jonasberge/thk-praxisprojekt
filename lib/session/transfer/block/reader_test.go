package block

import (
	"github.com/stretchr/testify/assert"
	"io"
	. "projekt/core/lib/util/buffer"
	"testing"
)

var Data = []byte("01234") // 0123 4
var BlockSize = Size(4)
var FirstBlock = newBlock(0, Data[0:4])
var LastBlock = newBlock(1, Data[4:])

func newBlock(index uint64, data []byte) *Block {
	return &Block{
		Index:    index,
		Data:     data,
		Checksum: ComputeChecksum(index, data),
	}
}

func createBlockReader() *Reader {
	reader := NewBuffer(Data)
	return NewBlockReader(reader, uint64(len(Data)), BlockSize)
}

func TestBlockReader_offset(t *testing.T) {
	r := createBlockReader()
	assert.EqualValues(t, 0, r.offset(0))
	assert.EqualValues(t, 4, r.offset(1))
	assert.EqualValues(t, 8, r.offset(2))
}

func TestBlockReader_amount(t *testing.T) {
	r := createBlockReader()
	assert.EqualValues(t, 4, r.amount(0))
	assert.EqualValues(t, 4, r.amount(1))
	assert.EqualValues(t, 3, r.amount(2))
	assert.EqualValues(t, 2, r.amount(3))
	assert.EqualValues(t, 1, r.amount(4))
	assert.EqualValues(t, 0, r.amount(5))
	assert.EqualValues(t, 0, r.amount(6))
}

func TestBlockReader_seek(t *testing.T) {
	r := createBlockReader()
	offset, err := r.seek(0)
	assert.Nil(t, err)
	assert.EqualValues(t, 0, offset)
	offset, err = r.seek(1)
	assert.Nil(t, err)
	assert.EqualValues(t, 4, offset)
	assert.Panics(t, func() { _, _ = r.seek(2) })
}

func TestBlockReader_ReadBlock(t *testing.T) {
	r := createBlockReader()
	block, err := r.ReadBlock(0)
	assert.Nil(t, err)
	assert.EqualValues(t, FirstBlock.Data, block.Data)
	block, err = r.ReadBlock(1)
	assert.Nil(t, err)
	assert.EqualValues(t, LastBlock.Data, block.Data)
	assert.Panics(t, func() { _, _ = r.ReadBlock(2) })
}

func expectNextBlock(t *testing.T, r *Reader, expectedData []byte) {
	block, err := r.NextBlock()
	assert.Nil(t, err)
	assert.EqualValues(t, expectedData, block.Data)
	// We assume that we are done if the data has length < BlockSize.
	// This is not necessarily true in general,
	// but in this case the last block has a different length than BlockSize.
	expectDone := uint64(len(expectedData)) < uint64(r.BlockSize())
	assert.Equal(t, expectDone, r.IsDone())
}

func expectNextBlockEOF(t *testing.T, r *Reader) {
	_, err := r.NextBlock()
	assert.ErrorIs(t, err, io.EOF)
}

func TestBlockReader_NextBlock(t *testing.T) {
	r := createBlockReader()
	expectNextBlock(t, r, FirstBlock.Data)
	expectNextBlock(t, r, LastBlock.Data)
	expectNextBlockEOF(t, r)
}

func TestBlockReader_RedoBlock(t *testing.T) {
	r := createBlockReader()
	queued, err := r.RedoBlock(1)
	assert.False(t, queued)
	assert.ErrorIs(t, err, IndexNotYetRead)
	expectNextBlock(t, r, FirstBlock.Data)
	queued, err = r.RedoBlock(0)
	assert.True(t, queued)
	assert.Nil(t, err)
	expectNextBlock(t, r, FirstBlock.Data)
	expectNextBlock(t, r, LastBlock.Data)
	queued, err = r.RedoBlock(1)
	assert.True(t, queued)
	assert.Nil(t, err)
	queued, err = r.RedoBlock(1)
	assert.False(t, queued)
	assert.Nil(t, err)
	expectNextBlock(t, r, LastBlock.Data)
	queued, err = r.RedoBlock(2)
	assert.False(t, queued)
	assert.ErrorIs(t, err, InvalidIndex)
	expectNextBlockEOF(t, r)
}
