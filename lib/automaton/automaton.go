package automaton

import "errors"

var (
	AmbiguousAtAndKey    = errors.New("ambiguous definition with triggering state in At and map key")
	MissingTriggerState  = errors.New("missing definition of triggering state in At or map key")
	AmbiguousTransitions = errors.New("ambiguous definition, cannot determine correct transition path")
	BadTransitionKey     = errors.New("transition key not defined in automaton")
	BadTransitionState   = errors.New("transition state for key not defined in automaton")
)

type TransitionMap = map[interface{}][]Transition
type Transitions TransitionMap

type Any = interface{}

type TransitionHandler func(*StateHandle, Any) (Any, error)
type TransitionHandlerFactory func(key, do Any) TransitionHandler
type TransitionHandlerInvoker func(Any) (Any, error)

type Automaton struct {
	HandlerFactory TransitionHandlerFactory
	Transitions    Transitions
}

type CompiledTransitionMap = map[interface{}]map[State]TransitionHandlerInvoker

type CompiledAutomaton struct {
	Transitions CompiledTransitionMap
}

var EmptyTransitionHandler = func(state *StateHandle, in Any) (out Any, err error) { return nil, nil }

func NewAutomaton(handlerFactory TransitionHandlerFactory, transitions Transitions) Automaton {
	return Automaton{
		HandlerFactory: handlerFactory,
		Transitions:    transitions,
	}
}

func (automaton Automaton) Compile(state *State) CompiledAutomaton {
	compiled := CompiledTransitionMap{}
	for key, transitions := range automaton.Transitions {
		at, stateInKey := key.(State)
		if stateInKey && len(transitions) > 1 {
			panic(AmbiguousTransitions)
		}

		handlers := map[State]TransitionHandlerInvoker{}
		for _, transition := range transitions {
			if stateInKey {
				if transition.At != nil {
					panic(AmbiguousAtAndKey)
				}
				transition.At = States{at}
			} else if transition.At == nil {
				panic(MissingTriggerState)
			}
			transition.Compile(state,
				func() TransitionHandler {
					return automaton.HandlerFactory(key, transition.Do)
				},
				func(state State, invoker TransitionHandlerInvoker) {
					if _, has := handlers[state]; has {
						panic(AmbiguousTransitions)
					}
					handlers[state] = invoker
				},
			)
		}

		compiled[key] = handlers
	}
	return CompiledAutomaton{
		Transitions: compiled,
	}
}

func (automaton CompiledAutomaton) Transition(key Any, state State, in Any) (out Any, err error) {
	sub, ok := automaton.Transitions[key]
	if !ok {
		return nil, BadTransitionKey
	}
	handler, ok := sub[state]
	if !ok {
		return nil, BadTransitionState
	}
	return handler(in)
}
