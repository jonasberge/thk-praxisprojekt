package device

import (
	"bytes"
	ed25519crypto "crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"github.com/flynn/noise"
	"github.com/oasisprotocol/curve25519-voi/primitives/ed25519"
	"github.com/oasisprotocol/curve25519-voi/primitives/x25519"
	"io"
	"math/big"
	"time"
)

// Key is the private key of a device.
type Key ed25519.PrivateKey

// X25519Key is the X25519 version of a Key.
type X25519Key []byte

func (k Key) X25519() X25519Key {
	return x25519.EdPrivateKeyToX25519(ed25519.PrivateKey(k))
}

// Identity is the public key of a device. It is used to identify the device.
type Identity ed25519.PublicKey

// X25519Identity is the X25519 version of an Identity.
type X25519Identity []byte

func (k Identity) X25519() X25519Identity {
	key, ok := x25519.EdPublicKeyToX25519(ed25519.PublicKey(k))
	if !ok {
		panic("failed to convert ed25519 public key to X25519 key")
	}
	return key
}

// KeyPair holds both a Key and its corresponding Identity.
type KeyPair struct {
	Private Key
	Public  Identity
}

// NoiseKeyPair represents the X25519 version of a KeyPair.
// It is used for noise encryption.
type NoiseKeyPair = noise.DHKey

// GenerateKeyPair generates a new KeyPair.
func GenerateKeyPair(reader io.Reader) (KeyPair, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(reader)
	if err != nil {
		return KeyPair{}, err
	}
	return KeyPair{
		Private: Key(privateKey),
		Public:  Identity(publicKey),
	}, nil
}

// Noise returns a NoiseKeyPair that holds the X25519 version of this KeyPair.
func (k *KeyPair) Noise() NoiseKeyPair {
	return noise.DHKey{
		Private: k.Private.X25519(),
		Public:  k.Public.X25519(),
	}
}

// Certificate derives a tls.Certificate from a KeyPair.
func (k *KeyPair) Certificate() (cert tls.Certificate, err error) {
	// Useful resource: https://golang.org/src/crypto/tls/generate_cert.go
	serialNumber, err := randomSerialNumber()
	if err != nil {
		return
	}
	template := x509.Certificate{
		SerialNumber: serialNumber,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour * 24 * 365),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	keyDer, _ := x509.MarshalPKCS8PrivateKey(ed25519crypto.PrivateKey(k.Private))
	certDer, _ := x509.CreateCertificate(
		rand.Reader, &template, &template, ed25519crypto.PublicKey(k.Public), ed25519crypto.PrivateKey(k.Private))
	keyBlock := &pem.Block{Type: "PRIVATE KEY", Bytes: keyDer}
	certBlock := &pem.Block{Type: "CERTIFICATE", Bytes: certDer}
	var certOut, keyOut bytes.Buffer
	_ = pem.Encode(&keyOut, keyBlock)
	_ = pem.Encode(&certOut, certBlock)
	return tls.X509KeyPair(certOut.Bytes(), keyOut.Bytes())
}

func randomSerialNumber() (*big.Int, error) {
	// https://golang.org/src/crypto/tls/generate_cert.go
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, serialNumberLimit)
}
