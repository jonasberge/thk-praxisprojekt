package secure

import (
	"errors"
	. "github.com/flynn/noise"
	"io"
	"net"
	. "projekt/core/lib/device"
	"projekt/core/lib/packet"
	"sync"
)

// TagSize is the size of the authentication tag
// of the noise.CipherAESGCM and noise.CipherChaChaPoly ciphers.
const TagSize = 16

// MessageMaxSize represents the maximum byte size of an encrypted Noise message.
const MessageMaxSize = (1 << 16) - 1

// PayloadMaxSize represents the maximum byte size of a message's payload (plaintext).
const PayloadMaxSize = MessageMaxSize - TagSize

// MessageSize returns the byte size of the encrypted ciphertext given the length of the plaintext.
func MessageSize(payloadLength int) int {
	return payloadLength + TagSize
}

// PayloadSize returns the byte size of the decrypted plaintext given the length of the ciphertext.
func PayloadSize(messageLength int) int {
	return messageLength - MessageSize(0)
}

type Conn interface {
	net.Conn

	// LocalIdentity returns the identity with which this connection communicates.
	LocalIdentity() Identity

	// RemoteIdentity returns the identity of the peer on the other side of the connection.
	RemoteIdentity() Identity
}

func WrapConn(identity Identity, peerIdentity Identity, conn net.Conn,
	writeCipher *CipherState, readCipher *CipherState) Conn {
	return &noiseConn{
		Conn:         conn,
		identity:     identity,
		peerIdentity: peerIdentity,
		writeCipher:  writeCipher,
		readCipher:   readCipher,
		writeMutex:   sync.Mutex{},
		readMutex:    sync.Mutex{},
	}
}

type noiseConn struct {
	net.Conn
	identity     Identity
	peerIdentity Identity
	writeCipher  *CipherState
	readCipher   *CipherState
	writeMutex   sync.Mutex
	readMutex    sync.Mutex
}

// Write writes the given payload p to the underlying connection.
// The length of the payload may not be larger than PayloadMaxSize.
func (c *noiseConn) Write(p []byte) (n int, err error) {
	size := MessageSize(len(p))
	if size > MessageMaxSize {
		err = errors.New("noise message is too long")
		return
	}
	// The encryption and write of a plaintext needs to be an atomic operation because
	// Noise enforces that messages are decrypted in the same order they were encrypted.
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()

	// Encrypt the data and append it to the length header.
	lenBuf := packet.Length(size).Bytes()
	message, err := c.writeCipher.Encrypt(lenBuf, lenBuf, p)
	if len(message)-len(lenBuf) != size {
		panic("unexpected ciphertext length")
	}
	if err != nil {
		return
	}

	// Write the resulting packet to the connection.
	m, err := c.Conn.Write(message)
	if err != nil {
		return
	}
	if m != len(message) {
		err = errors.New("failed to write entire payload")
		return
	}
	n = len(p)
	return
}

// Read reads an entire noise message into the given buffer.
// The buffer must have a size greater or equal to MessageMaxSize
// as it is used for storing the ciphertext that is read from the connection.
// The buffer may not be used or manipulated by another goroutine while this function is running
// since it is reused by the internal noise.CipherState for storing the plaintext after decryption.
func (c *noiseConn) Read(p []byte) (n int, err error) {
	if len(p) < PayloadMaxSize {
		err = errors.New("buffer is too small")
		return
	}
	c.readMutex.Lock()
	defer c.readMutex.Unlock()

	// Read a packet from the connection.
	lenBuf := make([]byte, packet.LengthSize)
	m, err := c.Conn.Read(lenBuf)
	if err != nil {
		return
	}
	if m != len(lenBuf) {
		err = errors.New("read invalid length field")
		return
	}

	// Decrypt the ciphertext and authenticate the length field.
	size, err := packet.DecodeLength(lenBuf)
	if err != nil {
		return
	}
	ct := p[:size]
	_, err = io.ReadFull(c.Conn, ct)
	if err != nil {
		return
	}
	plaintext, err := c.readCipher.Decrypt(ct[:0], size.Bytes(), ct)
	if err != nil {
		return
	}
	if len(plaintext) != PayloadSize(int(size)) {
		err = errors.New("unexpected plaintext length")
		return
	}
	n = len(plaintext)
	return
}

func (c *noiseConn) LocalIdentity() Identity {
	return c.identity
}

func (c *noiseConn) RemoteIdentity() Identity {
	return c.peerIdentity
}
