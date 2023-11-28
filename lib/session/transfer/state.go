package transfer

import . "projekt/core/lib/automaton"

const (
	initialized State = iota + 1
	done
	sendAbort
	aborted
	_firstState
)
