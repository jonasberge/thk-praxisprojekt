package relay

import (
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net"
	"projekt/core/lib/device"
	"strconv"
)

type Server struct {
	cert     tls.Certificate
	protocol *ServerProtocol
	log      *log.Logger

	port            int
	sessionPort     int
	listener        net.Listener
	sessionListener net.Listener
}

func NewServer(cert tls.Certificate, port, sessionPort int) *Server {
	if port == sessionPort {
		panic("server and session port must be different")
	}
	return &Server{
		cert:            cert,
		protocol:        NewServerProtocol(sessionPort),
		log:             log.New(log.Writer(), log.Prefix(), log.Flags()),
		port:            port,
		sessionPort:     sessionPort,
		listener:        nil,
		sessionListener: nil,
	}
}

func (s *Server) Listen() (err error) {
	err = s.protocol.Start()
	if err != nil {
		return
	}
	address := net.JoinHostPort("", strconv.Itoa(s.port))
	listener, err := tls.Listen("tcp", address, &tls.Config{
		Certificates: []tls.Certificate{s.cert},
		ClientAuth:   tls.RequestClientCert,
	})
	if err != nil {
		return
	}
	sessionListener, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(s.sessionPort)))
	if err != nil {
		_ = s.listener.Close()
		return
	}
	s.listener = listener
	s.sessionListener = sessionListener
	go s.handler()
	go s.sessionHandler()
	return
}

func (s *Server) handler() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// TODO Should we just continue on error? When is that possible?
			s.log.Println("failed to accept connection:", err)
			return
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	var err error
	defer func() {
		if err != nil {
			conn.Close()
		}
	}()
	tlsConn := conn.(*tls.Conn)
	err = tlsConn.Handshake()
	if err != nil {
		s.log.Println("failed to handshake with client:", err)
		return
	}
	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) != 1 {
		err = errors.New("expected exactly one client certificate")
		s.log.Println("connection canceled:", err)
		return
	}
	cert := state.PeerCertificates[0]
	publicKey, ok := cert.PublicKey.(ed25519.PublicKey)
	if !ok {
		err = errors.New("not an Ed25519 public key")
		s.log.Println("connection canceled:", err)
		return
	}
	s.protocol.AddClient(device.Identity(publicKey), tlsConn)
}

func (s *Server) sessionHandler() {
	for {
		conn, err := s.sessionListener.Accept()
		if err != nil {
			// TODO Should we just continue on error? When is that possible?
			s.log.Println("failed to accept session connection:", err)
			return
		}
		s.protocol.AddSessionClient(conn)
	}
}

func (s *Server) Close() (err error) {
	s.log.SetOutput(io.Discard)
	err = s.listener.Close()
	if err != nil {
		return
	}
	err = s.sessionListener.Close()
	return
}
