package automaton

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

const (
	initial State = iota + 1
	foo
	bar
)

type testHandler = func(state *StateHandle)
type testHandlerWithError = func(state *StateHandle) error

var emptyTestHandler = func(state *StateHandle) {}

func handlerFactory(_ Any, do Any) TransitionHandler {
	handle := do.(testHandler)
	return func(state *StateHandle, in Any) (out Any, err error) {
		handle(state)
		return
	}
}

func handlerWithErrorFactory(_ Any, do Any) TransitionHandler {
	handle := do.(testHandlerWithError)
	return func(state *StateHandle, in Any) (out Any, err error) {
		err = handle(state)
		return
	}
}

func newState() *State {
	state := new(State)
	*state = initial
	return state
}

func createAutomaton(transitions Transitions) Automaton {
	return Automaton{
		HandlerFactory: handlerFactory,
		Transitions:    transitions,
	}
}

func createAutomatonWithError(transitions Transitions) Automaton {
	return Automaton{
		HandlerFactory: handlerWithErrorFactory,
		Transitions:    transitions,
	}
}

func compileFunc(automaton Automaton) func() {
	return func() {
		automaton.Compile(newState())
	}
}

func compilationPanics(t *testing.T, err error, cases ...Transitions) {
	for _, transitions := range cases {
		assert.PanicsWithError(t, err.Error(), compileFunc(createAutomaton(transitions)))
	}
}

func automatonPanics(t *testing.T, err error, automaton CompiledAutomaton) {
	sub, ok := automaton.Transitions[initial]
	if !ok {
		panic("expected initial state as transition map key in test")
	}
	if _, ok := sub[initial]; !ok {
		panic("expected initial state as transition state key in test")
	}
	assert.PanicsWithError(t, err.Error(), func() {

		_, _ = automaton.Transition(initial, initial, nil)
	})
}

func transitionPanics(t *testing.T, err error, cases ...Transitions) {
	for _, transitions := range cases {
		state := newState()
		c := createAutomaton(transitions).Compile(state)
		automatonPanics(t, err, c)
	}
}

func transitionWithErrorPanics(t *testing.T, err error, cases ...Transitions) {
	for _, transitions := range cases {
		state := newState()
		c := createAutomatonWithError(transitions).Compile(state)
		automatonPanics(t, err, c)
	}
}

func TestAutomaton_AmbiguousAtAndKey(t *testing.T) {
	compilationPanics(t,
		AmbiguousAtAndKey,
		Transitions{
			initial: {
				Transition{
					At: States{initial},
				},
			},
		},
	)
}

func TestAutomaton_MissingTriggerState(t *testing.T) {
	compilationPanics(t,
		MissingTriggerState,
		Transitions{
			0: {
				Transition{
					To: initial,
				},
			},
		},
	)
}

func TestAutomaton_AmbiguousTransitions(t *testing.T) {
	compilationPanics(t,
		AmbiguousTransitions,
		Transitions{
			initial: {
				Transition{
					To: foo,
				},
				Transition{
					To: bar,
				},
			},
		},
		Transitions{
			0: {
				Transition{
					At: States{foo, initial},
					To: bar,
				},
				Transition{
					At: States{bar, initial},
					To: foo,
				},
			},
		},
	)
}

func TestAutomaton_OkWithoutDo(t *testing.T) {
	compilationPanics(t,
		OkWithoutDo,
		Transitions{
			initial: {
				Transition{
					Ok: States{foo},
				},
			},
		},
	)
}

func TestAutomaton_OkWithTo(t *testing.T) {
	compilationPanics(t,
		OkWithTo,
		Transitions{
			initial: {
				Transition{
					To: foo,
					Ok: States{foo},
					Do: emptyTestHandler,
				},
			},
		},
	)
}

func TestAutomaton_EmptyTransition(t *testing.T) {
	compilationPanics(t,
		EmptyTransition,
		Transitions{
			initial: {
				Transition{},
			},
		},
		Transitions{
			0: {
				Transition{
					At: States{initial},
				},
			},
		},
	)
}

func TestAutomaton_IllegalStateMutation(t *testing.T) {
	transitionPanics(t,
		IllegalStateMutation,
		Transitions{
			initial: {
				Transition{
					Do: func(state *StateHandle) {
						state.Set(foo)
					},
				},
			},
		},
		Transitions{
			initial: {
				Transition{
					Ok: States{foo},
					Do: func(state *StateHandle) {
						state.Set(bar)
					},
				},
			},
		},
	)
}

func TestAutomaton_MultipleStateMutations(t *testing.T) {
	transitionPanics(t,
		MultipleStateMutations,
		Transitions{
			initial: {
				Transition{
					Ok: States{foo, bar},
					Do: func(state *StateHandle) {
						state.Set(foo)
						state.Set(bar)
					},
				},
			},
		},
	)
}

func TestAutomaton_UnexpectedMutatedState(t *testing.T) {
	transitionPanics(t,
		UnexpectedMutatedState,
		Transitions{
			initial: {
				Transition{
					Ok: States{foo, bar},
					Do: func(state *StateHandle) {
						state.Set(foo)
						state.Is(initial)
					},
				},
			},
		},
	)
}

func TestAutomaton_NonTriggeringState(t *testing.T) {
	transitionPanics(t,
		NonTriggeringState,
		Transitions{
			initial: {
				Transition{
					Do: func(state *StateHandle) {
						state.Is(foo)
					},
				},
			},
		},
	)
}

func TestAutomaton_OkWithoutMutation(t *testing.T) {
	transitionWithErrorPanics(t,
		OkWithoutMutation,
		Transitions{
			initial: {
				Transition{
					Ok: States{foo},
					Do: func(state *StateHandle) error {
						return nil
					},
				},
			},
		},
	)
}

func TestAutomaton_NewStateWithError(t *testing.T) {
	transitionWithErrorPanics(t,
		NewStateWithError,
		Transitions{
			initial: {
				Transition{
					Ok: States{foo},
					Do: func(state *StateHandle) error {
						state.Set(foo)
						return errors.New("fail")
					},
				},
			},
		},
	)
}

func TestCompiledAutomaton_Transition(t *testing.T) {
	var path []int
	var step = func(breadcrumb int) { path = append(path, breadcrumb) }
	flag := false

	a := Automaton{
		HandlerFactory: handlerFactory,
		Transitions: Transitions{
			initial: {
				Transition{
					To: foo,
					Do: func(state *StateHandle) {
						step(1)
					},
				},
			},
			foo: {
				Transition{
					Ok: States{initial, bar},
					Do: func(state *StateHandle) {
						if !flag {
							step(2)
							state.Set(initial)
							flag = true
						} else {
							step(3)
							state.Set(bar)
						}
					},
				},
			},
			bar: {
				Transition{
					Do: func(state *StateHandle) {
						step(4)
					},
				},
			},
		},
	}

	c := a.Compile(newState())
	transition(t, &c, initial)
	transition(t, &c, foo)
	transition(t, &c, initial)
	transition(t, &c, foo)
	transition(t, &c, bar)

	assert.EqualValues(t, []int{1, 2, 1, 3, 4}, path)
}

func transition(t *testing.T, c *CompiledAutomaton, state State) {
	_, err := c.Transition(state, state, nil)
	assert.Nil(t, err)
}
