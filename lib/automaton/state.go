package automaton

import "errors"

var (
	IllegalStateMutation   = errors.New("mutating state is not allowed here")
	MultipleStateMutations = errors.New("mutating the state more than once is illegal")
	UnexpectedMutatedState = errors.New("did not expect a mutated state")
	NonTriggeringState     = errors.New("state is not a state which could have triggered the transition")
)

// State represents the state of an automaton.
// New automatons should use this type to represent their states.
// A state should not have the value 0. Define states starting from iota + 1.
// The zero value is reserved for detecting an invalid state.
type State int

// NoState represents an invalid, unset state.
var NoState = State(0)

func (s State) Is(state State) bool {
	return s == state
}

func (s State) IsNot(state State) bool {
	return !s.Is(state)
}

func (s State) IsAny(states ...State) bool {
	for _, other := range states {
		if s.Is(other) {
			return true
		}
	}
	return false
}

func (s State) IsNone(states ...State) bool {
	return !s.IsAny(states...)
}

type States []State

// StateHandle is a handle for transitioning to a new state.
// Changing the state more than once is an error and will trigger a panic.
// Not calling any method is an error too,
// when no destination state To is specified in the transition definition.
type StateHandle struct {
	state    *State
	original []State
	target   []State
	mutated  bool
}

func NewStateHandle(statePointer *State, originalStates []State, targetStates []State) (handle *StateHandle) {
	return &StateHandle{
		state:    statePointer,
		original: originalStates,
		target:   targetStates,
		mutated:  false,
	}
}

// Set sets the new state.
func (h *StateHandle) Set(newState State) {
	h.requireTarget(newState)
	h.mutate()
	*h.state = newState
}

// IsMutated returns if the underlying state was mutated
// by one of the functions of this StateHandle instance.
func (h *StateHandle) IsMutated() bool {
	return h.mutated
}

// Is checks if the input state is the specified state.
// It is an error to call this method after mutating the state.
// The passed state must be a state
// that was defined in the At field of the transition definition.
func (h *StateHandle) Is(state State) bool {
	// The state should not have been mutated.
	// The caller likely expects Is to operate on the input state At
	// and not the state that they changed with a previous function call.
	h.requireNotMutated()
	// The checked state must be one of the possible states.
	// Considering a state that is not possible is an error.
	h.requireOriginal(state)
	return h.state.Is(state)
}

// IsNot is the negation of a call to Is.
func (h *StateHandle) IsNot(state State) bool {
	return !h.Is(state)
}

// IsAny checks if any of the states return true when passed to Is.
func (h *StateHandle) IsAny(states ...State) bool {
	for _, other := range states {
		if h.Is(other) {
			return true
		}
	}
	return false
}

// IsNone checks if none of the states return true when passed to Is.
func (h *StateHandle) IsNone(states ...State) bool {
	return !h.IsAny(states...)
}

func (h *StateHandle) mutate() {
	if h.mutated {
		panic(MultipleStateMutations)
	}
	h.mutated = true
}

func (h *StateHandle) requireNotMutated() {
	if h.mutated {
		panic(UnexpectedMutatedState)
	}
}

func (h *StateHandle) requireOriginal(state State) {
	for _, original := range h.original {
		if state == original {
			return
		}
	}
	panic(NonTriggeringState)
}

func (h *StateHandle) requireTarget(state State) {
	for _, target := range h.target {
		if state == target {
			return
		}
	}
	panic(IllegalStateMutation)
}
