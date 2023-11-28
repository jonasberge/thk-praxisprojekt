package session

import (
	"errors"
	"fmt"
	"google.golang.org/protobuf/proto"
	"io"
	"log"
	"net"
	"projekt/core/lib/protocol"
	"projekt/core/lib/secure"
	"strings"
)

/*

Session
- manages a net.Conn
- all messages have type session.Message (oneof type)
- sends session.Message.Heartbeat every 20-ish seconds
- must receive a heartbeat within 40-ish seconds, otherwise closes the connection
- can open a new channelHandle which speaks a specific protocol
	* the channel_id is generated based on the socket type (initiator: 1, 3, ... or acceptor: 2, 4, 6)
	* the channel_type (or rather protocol_id) is defined by the used protocol: protocol.Protocol.Id()
	* returns a channelHandle instance with the following methods:
		- Write()

*/

var ErrClosed = errors.New("session is closed")
var ErrChannelClosed = errors.New("channel is closed")

type Session interface {
	// OpenChannel creates a channel on which the given protocol is spoken.
	// It blocks until the peer has accepted the channel with AcceptChannel
	// and the same protocol. or an error occurs.
	// TODO Close as first response means, that the peer does not speak the protocol. That needs to be revised.
	OpenChannel(protocol protocol.Protocol) (Channel, error)

	// AcceptChannel blocks until the peer opens a channel
	// with the given protocol or an unrecoverable error occurs.
	AcceptChannel(protocol protocol.Protocol) (Channel, error)

	// Close closes all its channels and the underlying connection.
	// TODO Explicitly close the session as well?
	Close() error
}

type Channel interface {
	// Write writes a proto.Message to the channel.
	// It must be understood by the underlying protocol.
	Write(message proto.Message) error

	// Read reads a proto.Message of the peer from the channel.
	Read() (proto.Message, error)

	// Close closes the channel.
	// TODO Get an acknowledge from the peer that they recognized that the channel was closed?
	Close() error

	// Protocol returns the protocol that is spoken on this Channel.
	Protocol() protocol.Protocol

	// Id returns the ChannelId of this Channel.
	Id() ChannelId

	// PayloadMaxSize returns the maximum binary size
	// of any marshalled Protobuf message that can be sent over this channel.
	PayloadMaxSize() int
}

// NewSession wraps a net.Conn in a Session instance.
func NewSession(conn secure.Conn, initiator bool) Session {
	s := &session{
		conn:       conn,
		factory:    NewChannelFactory(initiator),
		channels:   make(chan *channel),
		data:       make(chan *Data),
		inAccept:   make(chan ChannelId),
		waitAccept: make(chan waitAccept),
		inOpen:     make(chan doOpen),
		waitOpen:   make(chan waitOpen),
		done:       make(chan struct{}),
		cleaned:    make(chan struct{}),
	}
	go s.broker()
	go s.readHandler()
	return s
}

type session struct {
	conn       secure.Conn
	factory    *ChannelFactory
	channels   chan *channel
	data       chan *Data
	inAccept   chan ChannelId
	waitAccept chan waitAccept
	inOpen     chan doOpen
	waitOpen   chan waitOpen
	done       chan struct{}
	cleaned    chan struct{}
}

type waitAccept struct {
	channelId ChannelId
	accepted  chan struct{}
}

type doOpen struct {
	channelId  ChannelId
	protocolId protocol.Id
}

type waitOpen struct {
	protocolId protocol.Id
	opened     chan ChannelId
}

func (s *session) readHandler() {
	buf := make([]byte, secure.MessageMaxSize)
	for {
		n, err := s.conn.Read(buf)
		if err != nil {
			if err == net.ErrClosed {
				return
			}
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			if err == io.EOF {
				return
			}
			panic(fmt.Errorf("unhandled read error: %w", err))
		}
		data := buf[:n]
		message, err := UnmarshalMessage(data)
		if err != nil {
			log.Println("failed to unmarshal channel message")
			continue
		}
		switch {
		case message.IsData():
			s.data <- message.Content().(*Data)
		case message.OpensChannel():
			s.inOpen <- doOpen{
				channelId:  message.ChannelId(),
				protocolId: protocol.Id(message.Type()),
			}
		case message.AcceptsChannel():
			s.inAccept <- message.ChannelId()
		case message.ClosesChannel():

		}
	}
}

func (s *session) broker() {
	const Size = 4
	channels := make(map[ChannelId]*channel, Size)
	opens := make(map[protocol.Id]chan ChannelId, Size)
	accepts := make(map[ChannelId]chan struct{}, Size)
	// TODO: Close channel
	for {
		select {
		case open := <-s.waitOpen:
			if _, has := opens[open.protocolId]; has {
				// TODO i.e. we cannot accept a channel with the same protocol multiple times
				//  communicate this to the user of the API
				panic("waiting for opening of a channel which is already being waited for")
			}
			opens[open.protocolId] = open.opened
		case open := <-s.inOpen:
			if _, has := channels[open.channelId]; has {
				log.Println("received an open request for a channel that is already open")
				continue
			}
			opened, ok := opens[open.protocolId]
			if !ok {
				log.Println("received an open request which is not being waited for")
				// TODO: Communicate this to the peer.
				// TODO: Wait until it is being waited for?
				continue
			}
			opened <- open.channelId
			delete(opens, open.protocolId)
		case accept := <-s.waitAccept:
			if _, has := accepts[accept.channelId]; has {
				panic("waiting for an accept which is already being waited for")
			}
			if _, has := channels[accept.channelId]; has {
				panic("waiting for an accept of a channel that already exists")
			}
			accepts[accept.channelId] = accept.accepted
		case channelId := <-s.inAccept:
			if _, has := channels[channelId]; has {
				log.Println("received an accept for a channel that is already open")
				continue
			}
			accepted, ok := accepts[channelId]
			if !ok {
				log.Println("received an accept which is not being waited for")
				continue
			}
			close(accepted)
			delete(accepts, channelId)
		case ch := <-s.channels:
			if _, has := channels[ch.Id()]; has {
				panic("attempting to register a channel with an ID that is already opened")
			}
			channels[ch.Id()] = ch
		case data := <-s.data:
			ch, ok := channels[data.ChannelId]
			if !ok {
				log.Println("received data for a channel that is not opened:", data.ChannelId)
				continue
			}
			message, err := ch.handle.Protocol().NewMessage(data.MessageType)
			if err != nil {
				log.Println("failed to create a new message from received data:", err)
				continue
			}
			err = proto.Unmarshal(data.Payload, message)
			if err != nil {
				log.Println("failed to unmarshal received data:", err)
				continue
			}
			select {
			case ch.in <- message:
			case <-s.done:
				return
			}
		case <-s.done:
			_ = s.conn.Close()
			//for _, ch := range channels {
			//	// TODO close each channel concurrently.
			//	err := ch.Close()
			//	if err != nil {
			//		log.Printf("failed to close channel %v: %v", ch.Id(), err)
			//	}
			//}
			return
		}
	}
}

func (s *session) OpenChannel(protocol protocol.Protocol) (c Channel, err error) {
	handle := s.factory.CreateChannelHandle(protocol)
	openMessage := handle.OpenMessage()
	openData, err := proto.Marshal(openMessage)
	if err != nil {
		return
	}
	accepted := make(chan struct{})
	s.waitAccept <- waitAccept{handle.Id(), accepted}
	_, err = s.conn.Write(openData)
	if err != nil {
		return
	}
	// TODO Add timeout or context
	select {
	case <-accepted:
	case <-s.done:
		err = ErrClosed
		return
	}
	ch := &channel{
		handle: handle,
		conn:   s.conn,
		in:     make(chan proto.Message),
		next:   make(chan proto.Message),
		done:   make(chan struct{}),
	}
	go ch.handler()
	s.channels <- ch
	c = ch
	return
}

func (s *session) AcceptChannel(protocol protocol.Protocol) (c Channel, err error) {
	opened := make(chan ChannelId)
	s.waitOpen <- waitOpen{
		protocolId: protocol.Id(),
		opened:     opened,
	}
	var channelId ChannelId
	select {
	case channelId = <-opened:
	case <-s.done:
		err = ErrClosed
		return
	}
	handle := NewChannelHandle(protocol, channelId)
	acceptMessage := handle.AcceptMessage()
	acceptData, err := proto.Marshal(acceptMessage)
	if err != nil {
		return
	}
	_, err = s.conn.Write(acceptData)
	if err != nil {
		return
	}
	ch := &channel{
		handle: handle,
		conn:   s.conn,
		in:     make(chan proto.Message),
		next:   make(chan proto.Message),
		done:   make(chan struct{}),
	}
	go ch.handler()
	s.channels <- ch
	c = ch
	return
}

func (s *session) Close() error {
	close(s.done)
	return s.conn.Close()
}

type channel struct {
	handle ChannelHandle
	conn   secure.Conn
	in     chan proto.Message
	next   chan proto.Message
	done   chan struct{}
}

func (c *channel) handler() {
	queue := make([]proto.Message, 0, 16)
	for {
		if len(queue) > 0 {
			select {
			case c.next <- queue[0]:
				// FIXME this will move right in memory by 1 on every read.
				//  use a circular buffer or so.
				queue = queue[1:]
			case message := <-c.in:
				queue = append(queue, message)
			case <-c.done:
				return
			}
		} else {
			select {
			case message := <-c.in:
				queue = append(queue, message)
			case <-c.done:
				return
			}
		}
	}
}

func (c *channel) Write(message proto.Message) (err error) {
	select {
	case <-c.done:
		return ErrChannelClosed
	default:
	}
	data, err := c.handle.MarshalMessage(message)
	if err != nil {
		return
	}
	_, err = c.conn.Write(data)
	return
}

func (c *channel) Read() (message proto.Message, err error) {
	select {
	case message = <-c.next:
	case <-c.done:
		err = ErrChannelClosed
	}
	return
}

func (c *channel) Close() error {
	close(c.done)
	closeMessage := c.handle.CloseMessage()
	closeData, err := proto.Marshal(closeMessage)
	_, err = c.conn.Write(closeData)
	if err != nil {
		return err
	}
	// TODO wait for an ACK.
	return nil
}

func (c *channel) Protocol() protocol.Protocol {
	return c.handle.Protocol()
}

func (c *channel) Id() ChannelId {
	return c.handle.Id()
}

func (c *channel) PayloadMaxSize() int {
	return c.handle.PayloadMaxSize()
}
