package packet

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
)

var data1 = []byte("golden")
var data2 = []byte("gate")

func TestPacket_WriteTo(t *testing.T) {
	l, err := LengthOf(data1)
	assert.Nil(t, err)
	var b bytes.Buffer
	p := New(data1)
	n, err := p.WriteTo(&b)
	assert.Nil(t, err)
	assert.EqualValues(t, len(data1)+LengthSize, n)
	assert.EqualValues(t, l.Bytes(), b.Bytes()[:LengthSize])
	assert.EqualValues(t, data1, b.Bytes()[LengthSize:])
}

func TestSequence_WriteTo(t *testing.T) {
	l1, err := LengthOf(data1)
	assert.Nil(t, err)
	l2, err := LengthOf(data2)
	assert.Nil(t, err)
	var b bytes.Buffer
	p1 := New(data1)
	n1, err := p1.WriteTo(&b)
	assert.Nil(t, err)
	assert.EqualValues(t, len(data1)+LengthSize, n1)
	assert.EqualValues(t, l1.Bytes(), b.Bytes()[:LengthSize])
	assert.EqualValues(t, data1, b.Bytes()[LengthSize:])
	p2 := New(data2)
	n2, err := p2.WriteTo(&b)
	assert.Nil(t, err)
	assert.EqualValues(t, len(data2)+LengthSize, n2)
	assert.EqualValues(t, l2.Bytes(), b.Bytes()[n1:n1+LengthSize])
	assert.EqualValues(t, data2, b.Bytes()[n1+LengthSize:])
}

func TestDecode(t *testing.T) {
	var b bytes.Buffer
	_, err := New(data1).WriteTo(&b)
	assert.Nil(t, err)
	data, err := Decode(b.Bytes())
	assert.Nil(t, err)
	assert.EqualValues(t, data1, data)
}

func TestDecodeFrom(t *testing.T) {
	var b bytes.Buffer
	_, err := New(data1).WriteTo(&b)
	assert.Nil(t, err)
	data, err := DecodeFrom(&b)
	assert.Nil(t, err)
	assert.EqualValues(t, data1, data)
}
