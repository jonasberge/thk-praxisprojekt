package relay

import (
	"context"
	. "projekt/core/lib/device"
)

/*

struct SessionInfo {
	Description *SessionDescription
	PeerIdentity Identity
}

struct ServerProtocol {}
func NewServerProtocol(sessionPort int) *ServerProtocol
method Start()
method AddClient(identity Identity, conn net.Conn)
method AddSessionClient(conn net.Conn)
method Close() error

struct ClientProtocol {}
func NewClientProtocol(serverConn net.Conn) *ClientProtocol
method Start()
method Request(ctx context.Context, targetIdentity Identity, additionalData []byte) (SessionInfo, error)
method ReadAccept(ctx context.Context, targetIdentity Identity) ([]byte, error)
method ReadInvitation() (SessionInfo, []byte, error)
method Accept(context.Context, initiatorIdentity Identity, additionalData []byte) (error)
method Close() error

struct SessionClientProtocol {}
func NewSessionClientProtocol(sessionConn net.Conn) *SessionClientProtocol
method Authenticate(ctx context.Context, key []byte) (error)

*/

// StartCloser is an interface for a type that needs initialization in a Start method
// and can be closed after its purpose has been fulfilled with the Close method.
type StartCloser interface {
	// Start performs any necessary initialization and starts background handlers.
	// Generally it must be called before any other methods are used.
	Start(ctx context.Context) error
	// Close frees any resources and stops background handlers that have been started with Start.
	Close() error
}

// SessionInfo contains information about a session.
type SessionInfo struct {
	// Description contains general connection information about the session.
	Description *SessionDescription
	// PeerIdentity contains the identity of the peer of this session.
	PeerIdentity Identity
}

type identityHex = string
