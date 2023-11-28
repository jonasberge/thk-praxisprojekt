package automaton

import "errors"

var (
	OkWithTo          = errors.New("cannot have Ok and To within the same transition")
	OkWithoutDo       = errors.New("cannot have Ok without a Do handler")
	EmptyTransition   = errors.New("cannot have an empty transition")
	OkWithoutMutation = errors.New("specified Ok but did not mutate state in Do")
	NewStateWithError = errors.New("it is illegal to mutate the state when an error occurred") // TODO really?
)

type Transition struct {
	At []State
	Ok []State
	To State
	Do interface{}
}

func (transition Transition) Compile(state *State,
	createHandler func() TransitionHandler,
	callback func(State, TransitionHandlerInvoker)) {

	// if Ok is set then the state must be mutated.
	// not mutating the state in some branch is likely an error.
	mustMutateState := transition.Ok != nil

	if transition.Ok == nil {
		if transition.Do == nil && transition.To == NoState {
			panic(EmptyTransition)
		}
		transition.Ok = States{}
	} else if transition.To != NoState {
		panic(OkWithTo)
	} else if transition.Do == nil {
		panic(OkWithoutDo)
	}

	handler := EmptyTransitionHandler
	if transition.Do != nil {
		handler = createHandler()
	}

	for _, x := range transition.At {
		callback(x, func(in Any) (out Any, err error) {
			stateHandle := NewStateHandle(state, transition.At, transition.Ok)
			out, err = handler(stateHandle, in)
			if transition.To != NoState {
				*state = transition.To
			}
			if err != nil && stateHandle.IsMutated() {
				panic(NewStateWithError)
			}
			if err == nil && mustMutateState && !stateHandle.IsMutated() {
				// it's ok to not mutate when an error occurs
				panic(OkWithoutMutation)
			}
			return
		})
	}
}
