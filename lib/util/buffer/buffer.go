package buffer

import (
	"errors"
	"io"
)

var Closed = errors.New("buffer is closed")

type Buffer struct {
	Data   []byte
	cursor int
	closed bool
}

func NewBuffer(buffer []byte) *Buffer {
	return &Buffer{
		Data:   buffer,
		cursor: 0,
		closed: false,
	}
}

func (b *Buffer) Read(p []byte) (n int, err error) {
	if b.closed {
		return 0, Closed
	}
	n = copy(p, b.Data[b.cursor:])
	if n != len(p) {
		err = io.EOF
	}
	return
}

func (b *Buffer) Write(p []byte) (n int, err error) {
	if b.closed {
		return 0, Closed
	}
	if b.cursor+len(p) > len(b.Data) {
		panic("exceeding buffer size")
	}
	n = copy(b.Data[b.cursor:], p)
	if n != len(p) {
		err = io.EOF
	}
	return
}

func (b *Buffer) Seek(offset int64, whence int) (int64, error) {
	if b.closed {
		return 0, Closed
	}
	if whence != io.SeekStart {
		panic("used a whence different from io.SeekStart")
	}
	b.cursor = int(offset)
	return offset, nil
}

func (b *Buffer) Close() error {
	b.closed = true
	return nil
}
