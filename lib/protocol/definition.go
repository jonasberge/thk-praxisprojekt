package protocol

import . "projekt/core/lib/automaton"

// Definition defines how a protocol client's ReadMessage and WriteMessage functions operate.
// The logic is represented as a collection of transitions in a finite-state automaton.
// WriteMessage uses a Do handler of the type WriteMessageHandler.
// ReadMessage uses a Do handler of the type ReadMessageHandler.
type Definition struct {
	// Protocol defines the messages and types that are used in this protocol.
	Protocol Protocol

	// WriteFirst specifies that this protocol definition
	// is designed to immediately write any pending messages.
	// More specifically the states that trigger the transitions in WriteMessage
	// are incapable of triggering any states in the transitions of ReadMessage.
	// Those states are "write-only" states in which messages cannot be read.
	// If this fields is set to true, then Client.MustWrite returns true
	// whenever there is a message to write i.e. Client.CanWrite returns true as well.
	WriteFirst bool

	// InitialState represents the initial state of this protocol.
	InitialState State

	// FinalState represents the final state of this protocol.
	// Client.IsDone will return true when this state is reached.
	FinalState State

	// WriteMessage contains the transitions for when a message is written.
	// The triggering State should be the key in the automaton.TransitionMap.
	WriteMessage Transitions

	// ReadMessage contains the transitions for when a message is read.
	// The message type must be the key in the automaton.TransitionMap.
	ReadMessage Transitions
}
