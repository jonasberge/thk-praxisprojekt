package protocol

import (
	"google.golang.org/protobuf/proto"
	. "projekt/core/lib/automaton"
	"sync"
)

// Handler is a protocol handler that represents one side of protocol communication.
// Usually one Handler exchanges messages with exactly one other Handler.
// It must be safe to call any of the functions in this interface concurrently.
type Handler interface {
	// WriteMessage returns the next message of this client in the protocol.
	// It needs to be transmitted to another Handler and read with their ReadMessage function.
	// If there is no message to write, this function must return a NothingToWrite error.
	// A different error may be returned if creation of the message fails
	// or another error occurred during protocol execution.
	WriteMessage() (message proto.Message, err error)

	// ReadMessage accepts a message that was written with another Handler's WriteMessage function.
	// If the passed message was unexpected in the protocol,
	// this function needs to return an UnexpectedMessage error.
	// A different error may be returned if processing of the message fails
	// or another error occurred during protocol execution.
	ReadMessage(message proto.Message) (err error)

	// CanWrite signals if there is data that can be written via WriteMessage.
	// The result may change after a call to WriteMessage or ReadMessage
	// as only these functions mutate internal protocol state.
	CanWrite() bool

	// MustWrite signals if there is data that must be written via WriteMessage
	// before the next message can be read via ReadMessage.
	// ReadMessage may block until this condition has been satisfied.
	MustWrite() bool

	// IsDone signals if the underlying protocol finished execution.
	IsDone() bool
}

// Client implements protocol.Handler.
// It represents one side of protocol communication
// by adhering to the rules of a protocol's Definition.
type Client struct {
	protocol     Protocol
	writeFirst   bool
	initialState State
	finalState   State

	state *State
	mutex sync.Mutex

	writer  CompiledAutomaton
	reader  CompiledAutomaton
	writeAt map[State]struct{}
}

type WriteMessageHandler = func(state *StateHandle) (message proto.Message, err error)
type ReadMessageHandler = func(state *StateHandle, message proto.Message) (err error)

func writeMessageHandlerFactory(_, do Any) TransitionHandler {
	handle := do.(WriteMessageHandler)
	return func(state *StateHandle, in Any) (out Any, err error) {
		out, err = handle(state)
		if _, ok := out.(proto.Message); !ok {
			panic("written message needs be a protobuf message")
		}
		return
	}
}

func readMessageHandlerFactory(_, do Any) TransitionHandler {
	handle := do.(ReadMessageHandler)
	return func(state *StateHandle, in Any) (out Any, err error) {
		message, ok := in.(proto.Message)
		// TODO: listen for specific messages and trigger registered callbacks
		if !ok {
			panic("read message needs to be a protobuf message")
		}
		err = handle(state, message)
		return
	}
}

var WriteMessageHandlerFactory TransitionHandlerFactory = writeMessageHandlerFactory
var ReadMessageHandlerFactory TransitionHandlerFactory = readMessageHandlerFactory

// NewClient creates a new protocol client from a protocol Definition.
func NewClient(definition *Definition) *Client {
	if definition.InitialState == NoState {
		panic("initial state is required in protocol definition")
	}
	if definition.FinalState == NoState {
		panic("final state is required in protocol definition")
	}

	state := new(State)
	*state = definition.InitialState

	writer := NewAutomaton(WriteMessageHandlerFactory, definition.WriteMessage).Compile(state)
	reader := NewAutomaton(ReadMessageHandlerFactory, definition.ReadMessage).Compile(state)

	writeAt := make(map[State]struct{}, len(writer.Transitions))
	for key := range writer.Transitions {
		writeAt[key.(State)] = struct{}{}
	}

	return &Client{
		protocol:     definition.Protocol,
		writeFirst:   definition.WriteFirst,
		initialState: definition.InitialState,
		finalState:   definition.FinalState,
		state:        state,
		mutex:        sync.Mutex{},
		writer:       writer,
		reader:       reader,
		writeAt:      writeAt,
	}
}

// WriteMessage implements protocol.Handler's WriteMessage function.
// It is safe to call this function concurrently.
func (s *Client) WriteMessage() (message proto.Message, err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	out, err := s.writer.Transition(s.State(), s.State(), nil)
	if err == BadTransitionKey || err == BadTransitionState {
		err = NothingToWrite
		return
	}

	message = out.(proto.Message)
	return
}

// ReadMessage implements protocol.Handler's ReadMessage function.
// It is safe to call this function concurrently.
func (s *Client) ReadMessage(message proto.Message) (err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	messageType := s.protocol.TypeOf(message)
	_, err = s.reader.Transition(messageType, s.State(), message)
	if err == BadTransitionKey || err == BadTransitionState {
		err = UnexpectedMessage
	}
	return
}

// CanWrite implements protocol.Handler's NeedsWrite function.
func (s *Client) CanWrite() bool {
	_, needsWrite := s.writeAt[s.State()]
	return needsWrite
}

// MustWrite implements protocol.Handler's MustWrite function.
// Calls to ReadMessage will block until this function returns false.
func (s *Client) MustWrite() bool {
	return s.writeFirst && s.CanWrite()
}

// IsDone implements protocol.Handler's IsDone function.
func (s *Client) IsDone() bool {
	return s.State() == s.finalState
}

// State returns the current protocol state.
// It is safe to call this function concurrently.
func (s *Client) State() State {
	return *s.state // thread-safe copy
}
