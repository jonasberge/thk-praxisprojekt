package packet

import (
	"errors"
	"io"
)

// Interface contains the functions a generic packet must provide.
type Interface interface {
	io.WriterTo
	Size() int64
}

// Packet represents a network packet.
type Packet []byte

// New constructs a new Packet from a sequence of bytes.
func New(data []byte) Packet {
	return data
}

// WriteTo writes a packet to an io.Writer.
func (p Packet) WriteTo(w io.Writer) (n int64, err error) {
	length, err := LengthOf(p)
	if err != nil {
		return
	}
	n1, err := w.Write(length.Bytes())
	if err != nil {
		return
	}
	n2, err := w.Write(p)
	n = int64(n1 + n2)
	return
}

// Decode decodes a packet from a sequence of bytes and returns the resulting Packet.
func Decode(raw []byte) (packet Packet, err error) {
	if len(raw) < LengthSize {
		err = errors.New("missing length header")
		return
	}
	length, err := DecodeLength(raw[:LengthSize])
	if err != nil {
		return
	}
	if len(raw)-LengthSize != int(length) {
		err = errors.New("length of remaining bytes does not conform to header value")
		return
	}
	packet = New(raw[LengthSize:])
	return
}

// DecodeFrom reads and decodes a whole Packet from an io.Reader and returns it.
func DecodeFrom(r io.Reader) (packet Packet, err error) {
	var header [LengthSize]byte
	_, err = io.ReadFull(r, header[:])
	if err != nil {
		return
	}
	length, err := DecodeLength(header[:])
	if err != nil {
		return
	}
	packet = make([]byte, length)
	_, err = io.ReadFull(r, packet)
	return
}

// Size returns the number of raw bytes that would encode this packet.
// It consists of the Length header and the stream of bytes.
func (p Packet) Size() int64 {
	return int64(LengthSize + len(p))
}

// Sequence is a sequence of multiple Packet's in a row.
type Sequence []Packet

// NewSequence constructs a Sequence from multiple byte slices.
func NewSequence(packets ...[]byte) Sequence {
	sequence := make(Sequence, len(packets))
	for i, packet := range packets {
		sequence[i] = packet
	}
	return sequence
}

// WriteTo writes a Sequence to an io.Writer.
func (p Sequence) WriteTo(w io.Writer) (n int64, err error) {
	for _, packet := range p {
		var m int64
		m, err = packet.WriteTo(w)
		n += m
		if err != nil {
			return
		}
	}
	return
}

// Size returns the number of raw bytes that would encode this sequence of packets.
// It is the sum of the calls to Packet.Size of each packet in this sequence.
func (p Sequence) Size() (size int64) {
	for _, packet := range p {
		size += packet.Size()
	}
	return
}
