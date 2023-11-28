package secure

import (
	. "github.com/flynn/noise"
)

// HandshakeXK1fallback is used for establishing a connection on the local network.
var HandshakeXK1fallback = HandshakePattern{
	Name:                 "XK1fallback",
	InitiatorPreMessages: []MessagePattern{MessagePatternS},
	ResponderPreMessages: []MessagePattern{MessagePatternE},
	Messages: [][]MessagePattern{
		// Note that se and es are swapped here.
		//  Compare noise.HandshakeXX and noise.HandshakeXXfallback for reference.
		{MessagePatternE, MessagePatternDHEE, MessagePatternDHSE},
		{MessagePatternS, MessagePatternDHES},
	},
}
