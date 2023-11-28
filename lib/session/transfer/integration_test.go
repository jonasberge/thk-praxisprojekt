package transfer

import (
	"github.com/stretchr/testify/assert"
	"math/rand"
	"projekt/core/lib/session/transfer/block"
	. "projekt/core/lib/util/buffer"
	"testing"
)

var MinDataLen = 32
var MaxDataLen = 256
var MinBlockSize = 1
var MaxBlockSize = 256 + 64

func randBetween(min int, max int) int {
	// FIXME: This always uses the same seed.
	return rand.Intn(max+1-min) + min
}

func createReaderWriter() (in []byte, out []byte, reader *block.Reader, writer *block.Writer) {
	blockSize := block.Size(randBetween(MinBlockSize, MaxBlockSize))
	size := uint64(randBetween(MinDataLen, MaxDataLen))
	in = make([]byte, size)
	rand.Read(in)
	out = make([]byte, size)
	reader = block.NewBlockReader(NewBuffer(in), size, blockSize)
	writer = block.NewWriter(NewBuffer(out), size, blockSize)
	return
}

func Test_BlockReader_BlockWriter(t *testing.T) {
	for k := 0; k < 16; k++ {
		i, o, r, w := createReaderWriter()
		for r.HasData() {
			assert.False(t, w.IsDone())
			b, err := r.NextBlock()
			assert.Nil(t, err)
			err = w.WriteBlock(b)
			assert.Nil(t, err)
		}
		assert.True(t, w.IsDone())
		assert.EqualValues(t, i, o)
	}
}
