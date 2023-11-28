package relay

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"github.com/flynn/noise"
	"net"
	"projekt/core/cmd/base"
	"projekt/core/lib/device"
	"projekt/core/lib/local"
	"projekt/core/lib/secure"
	"strconv"
)

type Client struct {
	host     string
	protocol *ClientProtocol
	key      device.KeyPair
}

func Dial(key device.KeyPair, host string, port string) (client *Client, err error) {
	cert, err := key.Certificate()
	if err != nil {
		return
	}
	addr := net.JoinHostPort(host, port)
	conn, err := tls.Dial("tcp", addr, &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true, // TODO verify the server
	})
	if err != nil {
		return
	}
	protocol := NewClientProtocol(conn)
	err = protocol.Start(context.TODO()) // TODO
	if err != nil {
		return
	}
	client = &Client{
		host:     host,
		protocol: protocol,
		key:      key,
	}
	return
}

func (c *Client) Request(ctx context.Context, targetIdentity device.Identity) (conn secure.Conn, err error) {
	hs, err := noise.NewHandshakeState(noise.Config{
		CipherSuite:   base.CipherSuite, // TODO let the user configure this
		Random:        rand.Reader,
		Pattern:       noise.HandshakeKK,
		Initiator:     true,
		StaticKeypair: c.key.Noise(),
		PeerStatic:    targetIdentity.X25519(),
	})
	if err != nil {
		return
	}
	message, _, _, err := hs.WriteMessage(nil, nil)
	if err != nil {
		return
	}
	info, err := c.protocol.Request(ctx, targetIdentity, message)
	if err != nil {
		return
	}
	if !bytes.Equal(targetIdentity, info.PeerIdentity) {
		err = errors.New("unexpected peer identity")
		return
	}
	// TODO concurrently: join session and wait for accept
	ad, err := c.protocol.ReadAccept(ctx, targetIdentity)
	_, c1, c2, err := hs.ReadMessage(nil, ad)
	if err != nil {
		return
	}
	addr := net.JoinHostPort(c.host, strconv.Itoa(int(info.Description.Port)))
	sessConn, err := net.Dial("tcp", addr)
	if err != nil {
		return
	}
	session := NewSessionClientProtocol(sessConn)
	clientConn, err := session.Authenticate(info.Description.Key)
	if err != nil {
		return
	}
	conn = secure.WrapConn(c.key.Public, info.PeerIdentity, clientConn, c1, c2)
	return
}

func (c *Client) Accept(trust local.TrustStore) (secure.Conn, error) {
	for {
		// TODO do Noise handshake
		info, ad, err := c.protocol.ReadInvitation()
		if err != nil {
			return nil, err
		}
		if !trust.IsTrusted(info.PeerIdentity) {
			// Ignore requests of untrusted peers
			continue
		}
		hs, err := noise.NewHandshakeState(noise.Config{
			CipherSuite:   base.CipherSuite, // TODO let the user configure this
			Random:        rand.Reader,
			Pattern:       noise.HandshakeKK,
			StaticKeypair: c.key.Noise(),
			PeerStatic:    info.PeerIdentity.X25519(),
		})
		if err != nil {
			return nil, err
		}
		_, _, _, err = hs.ReadMessage(nil, ad)
		if err != nil {
			return nil, err
		}
		message, c1, c2, err := hs.WriteMessage(nil, nil)
		if err != nil {
			return nil, err
		}
		err = c.protocol.Accept(info.PeerIdentity, message)
		if err != nil {
			return nil, err
		}
		addr := net.JoinHostPort(c.host, strconv.Itoa(int(info.Description.Port)))
		sessConn, err := net.Dial("tcp", addr)
		if err != nil {
			return nil, err
		}
		session := NewSessionClientProtocol(sessConn)
		clientConn, err := session.Authenticate(info.Description.Key)
		if err != nil {
			return nil, err
		}
		conn := secure.WrapConn(c.key.Public, info.PeerIdentity, clientConn, c2, c1)
		return conn, err
	}
}
