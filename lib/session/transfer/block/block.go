package block

import (
	"crypto/sha256"
	"encoding/binary"
)

type Size uint64

func (s Size) BlockCount(size uint64) uint64 {
	if size == 0 {
		return 0
	}
	return (size-1)/uint64(s) + 1
}

func (s Size) Offset(index uint64) uint64 {
	return index * uint64(s)
}

// Index returns the index of a specific offset.
// Note that it does not check if the offset is a valid offset.
func (s Size) Index(offset uint64) uint64 {
	return offset / uint64(s)
}

func (s Size) Amount(offset uint64, size uint64) uint64 {
	if offset >= size {
		return 0
	}
	remaining := size - offset
	return min(uint64(s), remaining)
}

func ComputeChecksum(index uint64, data []byte) []byte {
	position := make([]byte, 8)
	binary.BigEndian.PutUint64(position, index)
	hash := sha256.New()
	hash.Write(position)
	hash.Write(data)
	return hash.Sum(nil)
}
