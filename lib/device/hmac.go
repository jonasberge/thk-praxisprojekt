package device

import (
	"io"
)

// HmacKeySize is the byte size of an HMACKey.
const HmacKeySize = 32

// HMACKey represents a device's HMAC key.
type HMACKey []byte

// GenerateHMACKey generates a new HMACKey.
func GenerateHMACKey(randSource io.Reader) (key HMACKey, err error) {
	key = make([]byte, HmacKeySize)
	_, err = io.ReadFull(randSource, key)
	if err != nil {
		key = nil
	}
	return
}
