package block

import (
	"bytes"
	"errors"
	"io"
)

var (
	BadBlockData         = errors.New("block data is either too large or too slow")
	WriterBoundsExceeded = errors.New("block offset behind end of writer")
	AlreadyWritten       = errors.New("block offset has already been written")
	BadBlockChecksum     = errors.New("the block's checksum is incorrect")
)

type WriteSeekCloser interface {
	io.Writer
	io.Seeker
	io.Closer
}

type Writer struct {
	writer    WriteSeekCloser
	totalSize uint64
	blockSize Size

	written      []bool
	writtenCount uint64
}

func NewWriter(writer WriteSeekCloser, size uint64, blockSize Size) *Writer {
	return &Writer{
		writer:       writer,
		totalSize:    size,
		blockSize:    blockSize,
		written:      make([]bool, blockSize.BlockCount(size)),
		writtenCount: 0,
	}
}

func (w *Writer) Writer() WriteSeekCloser {
	return w.writer
}

func (w *Writer) TotalSize() uint64 {
	return w.totalSize
}

func (w *Writer) BlockSize() Size {
	return w.blockSize
}

func (w *Writer) BlockCount() uint64 {
	return w.blockSize.BlockCount(w.totalSize)
}

func (w *Writer) IsDone() bool {
	return w.writtenCount == w.BlockCount()
}

func (w *Writer) WriteBlock(block *Block) (err error) {
	err = w.checkBlock(block)
	if err != nil {
		return
	}
	err = w.seek(block.Index)
	if err != nil {
		return
	}
	err = w.write(block.Data)
	if err != nil {
		return
	}
	w.written[block.Index] = true
	w.writtenCount += 1
	return
}

func (w *Writer) Close() error {
	return w.writer.Close()
}

func (w *Writer) checkBlock(block *Block) error {
	offset := w.offset(block.Index)
	if offset >= w.totalSize {
		return WriterBoundsExceeded
	}
	if uint64(len(block.Data)) != w.amount(offset) {
		return BadBlockData
	}
	if w.written[block.Index] {
		return AlreadyWritten
	}
	expected := ComputeChecksum(block.Index, block.Data)
	if !bytes.Equal(expected, block.Checksum) {
		return BadBlockChecksum
	}
	return nil
}

func (w *Writer) seek(index uint64) error {
	offset := w.offset(index)
	position, err := w.writer.Seek(int64(offset), io.SeekStart)
	if err != nil {
		return err
	}
	if position != int64(offset) {
		panic("seek position does not equal requested offset")
	}
	return nil
}

func (w *Writer) write(data []byte) error {
	n, err := w.writer.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		panic("unexpected amount of bytes written")
	}
	return nil
}

func (w *Writer) offset(index uint64) uint64 {
	return w.blockSize.Offset(index)
}

func (w *Writer) index(offset uint64) uint64 {
	return w.blockSize.Index(offset)
}

func (w *Writer) amount(offset uint64) uint64 {
	return w.blockSize.Amount(offset, w.totalSize)
}
