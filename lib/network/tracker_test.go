package network

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTracker_Listen_Present(t *testing.T) {
	r := NewTracker()
	present, _, err := r.Listen(context.TODO())
	assert.Nil(t, err)
	for _, n := range present {
		// manually check that the output corresponds to your connected networks
		fmt.Println(n)
	}
}
