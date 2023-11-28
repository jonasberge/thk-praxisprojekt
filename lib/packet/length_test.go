package packet

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLength_Bytes(t *testing.T) {
	d := []byte("bridge")
	l1, err := LengthOf(d)
	assert.Nil(t, err)
	assert.EqualValues(t, len(d), l1)
	l2, err := DecodeLength(l1.Bytes())
	assert.Nil(t, err)
	assert.EqualValues(t, len(d), l2)
}
