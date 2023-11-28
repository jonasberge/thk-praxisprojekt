package base

import (
	"crypto/rand"
	"encoding/hex"
	"github.com/flynn/noise"
	"io"
	"log"
	mathRand "math/rand"
	"projekt/core/lib/device"
	"projekt/core/lib/session/transfer/block"
	"time"
)

var CipherSuite = noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashSHA256)

var (
	IdentitySize  = 32
	DeviceKeySize = 32
	Port          = 23520
)

var (
	BlockSize block.Size = 1 << 15
	Timeout              = 30 * time.Second
)

type TrustAny struct{}

func (t *TrustAny) IsTrusted(identity device.Identity) bool {
	return true
}

func (t *TrustAny) IsX25519Trusted(identity device.X25519Identity) bool {
	return true
}

type TrustNone struct{}

func (t *TrustNone) IsTrusted(identity device.Identity) bool {
	return false
}

func (t *TrustNone) IsX25519Trusted(identity device.X25519Identity) bool {
	return false
}

func ToHex(identity device.Identity) string {
	return hex.EncodeToString(identity)
}

func FromHex(h string) (device.Identity, error) {
	return hex.DecodeString(h)
}

func CryptoRandom() io.Reader {
	return rand.Reader
}

func PredictableRandom(seed int64) io.Reader {
	return mathRand.New(mathRand.NewSource(seed))
}

func GenerateKeys(randSource io.Reader) (device.KeyPair, device.HMACKey) {
	identityKey, err := device.GenerateKeyPair(randSource)
	if err != nil {
		log.Fatalln("failed to generate identity key:", err)
	}
	hmacKey, err := device.GenerateHMACKey(randSource)
	if err != nil {
		log.Fatalln("failed to generate hmac key:", err)
	}
	log.Println("ED25519-IDENTITY", ToHex(identityKey.Public))
	log.Println("X25519-IDENTITY ", ToHex(identityKey.Noise().Public))
	log.Println("HMAC-KEY        ", hex.EncodeToString(hmacKey))
	return identityKey, hmacKey
}

func DecodeHex(hex string, expectedSize int) (data []byte) {
	data, err := FromHex(hex)
	if err != nil {
		log.Fatalln("failed to decode hex-encoded device identity:", err)
	}
	if len(data) != expectedSize {
		log.Fatalf("identity must be %v bytes long\n", expectedSize)
	}
	return
}
