package block

import (
	"errors"
	"io"
)

var (
	InvalidIndex    = errors.New("invalid block index")
	IndexNotYetRead = errors.New("block index was not read yet")
)

type ReaderFactory struct {
	Config Size
}

func NewReaderFactory(config Size) *ReaderFactory {
	return &ReaderFactory{
		Config: config,
	}
}

func (f *ReaderFactory) CreateBlockReader(reader ReadSeekCloser, size uint64) *Reader {
	return NewBlockReader(reader, size, f.Config)
}

type ReadSeekCloser io.ReadSeekCloser

type Reader struct {
	reader    ReadSeekCloser
	totalSize uint64
	blockSize Size

	nextIndex  uint64   // offset of next block
	redoBlocks []uint64 // these block offsets should be redone before continuing
	// TODO move redoBlocks into the protocol.
}

func NewBlockReader(reader ReadSeekCloser, size uint64, blockSize Size) *Reader {
	return &Reader{
		reader:     reader,
		totalSize:  size,
		blockSize:  blockSize,
		nextIndex:  0,
		redoBlocks: make([]uint64, 0),
	}
}

func (r *Reader) TotalSize() uint64 {
	return r.totalSize
}

func (r *Reader) BlockSize() Size {
	return r.blockSize
}

func (r *Reader) BlockCount() uint64 {
	return r.blockSize.BlockCount(r.totalSize)
}

func (r *Reader) NextBlockIndex() uint64 {
	return r.nextIndex
}

func (r *Reader) IsDone() bool {
	return len(r.redoBlocks) == 0 &&
		r.offset(r.nextIndex) >= r.totalSize
}

func (r *Reader) HasData() bool {
	return !r.IsDone()
}

// NextBlock reads the next block and returns it.
// This function may not be called concurrently.
func (r *Reader) NextBlock() (block *Block, err error) {
	if r.IsDone() {
		return nil, io.EOF
	}
	blockIndex := r.nextIndex
	if len(r.redoBlocks) > 0 {
		blockIndex = r.redoBlocks[0]
	}
	block, err = r.ReadBlock(blockIndex)
	if err != nil {
		block = nil
		return
	}
	// TODO: test that the index is not incremented when an error occurs
	if len(r.redoBlocks) > 0 {
		r.redoBlocks = r.redoBlocks[1:]
	} else {
		r.nextIndex += 1
	}
	return
}

func (r *Reader) RedoBlock(index uint64) (queued bool, err error) {
	if r.offset(index) >= r.totalSize {
		return false, InvalidIndex
	}
	if index >= r.nextIndex {
		return false, IndexNotYetRead
	}
	for _, other := range r.redoBlocks {
		if index == other {
			return false, nil
		}
	}
	r.redoBlocks = append(r.redoBlocks, index)
	return true, nil
}

func (r *Reader) ReadBlock(index uint64) (block *Block, err error) {
	offset, err := r.seek(index)
	if err != nil {
		return
	}
	data, err := r.read(offset)
	if err != nil {
		return
	}
	block = &Block{
		Index:    index,
		Data:     data,
		Checksum: ComputeChecksum(index, data),
	}
	return
}

func (r *Reader) Close() error {
	return r.reader.Close()
}

func (r *Reader) seek(index uint64) (offset uint64, err error) {
	offset = r.offset(index)
	if offset >= r.totalSize {
		panic("block index behind end of reader")
	}
	position, err := r.reader.Seek(int64(offset), io.SeekStart)
	if err != nil {
		return
	}
	if uint64(position) != offset {
		panic("seek position does not equal requested offset")
	}
	return
}

func (r *Reader) read(offset uint64) (buffer []byte, err error) {
	amount := r.amount(offset)
	// TODO do not allocate on every read
	//  maybe the caller should supply their buffer
	//  implement a buffer pool for reusing buffers
	buffer = make([]byte, amount)
	n, err := io.ReadFull(r.reader, buffer)
	if err != nil {
		return
	}
	if uint64(n) != amount {
		panic("unexpect amount of bytes read")
	}
	return
}

func (r *Reader) offset(index uint64) uint64 {
	return r.blockSize.Offset(index)
}

func (r *Reader) amount(offset uint64) uint64 {
	return r.blockSize.Amount(offset, r.totalSize)
}

func min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}
