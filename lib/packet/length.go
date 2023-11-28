package packet

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Length represents the length of a Packet.
// It must be encoded with the Bytes method to ensure consistency on the wire.
type Length uint16

// LengthSize is the byte size of a Length.
const LengthSize = 2

// MaxLength is the maximum length of a Packet.
const MaxLength = (1 << (LengthSize << 3)) - 1

// LengthOf creates a Length instance from the passed block of data.
// The instance holds the length that is returned by a call to len.
// It returns an error if the length is larger than MaxLength.
func LengthOf(data []byte) (Length, error) {
	if len(data) > MaxLength {
		return 0, fmt.Errorf("the data may not exceed %v bytes", MaxLength)
	}
	return Length(len(data)), nil
}

// DecodeLength decodes the Length that is encoded in a sequence of two bytes.
// It must have been encoded with the Length.Bytes method.
func DecodeLength(raw []byte) (Length, error) {
	if len(raw) != LengthSize {
		return 0, errors.New(fmt.Sprintf("a length field must be %v bytes long", LengthSize))
	}
	return Length(binary.BigEndian.Uint16(raw)), nil
}

// Bytes encodes a Length to two bytes with big endian encoding.
func (l Length) Bytes() []byte {
	bytes := make([]byte, 2)
	binary.BigEndian.PutUint16(bytes, uint16(l))
	return bytes
}
