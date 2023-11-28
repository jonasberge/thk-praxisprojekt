package transfer

import (
	"errors"
	"google.golang.org/protobuf/proto"
	. "projekt/core/lib/automaton"
	"projekt/core/lib/protocol"
	"projekt/core/lib/session/transfer/block"
)

const (
	inPrologue = iota + _firstState
	inProgress
	needAcknowledge
	transferred
	needDataReceived
)

var (
	DuplicateBlockReceived = errors.New("block receipt is already acknowledged")
	RedoForUnwrittenBlock  = errors.New("redo requested for block that was not yet written")
	RedoForReceivedBlock   = errors.New("redo requested for acknowledged block")
	DuplicateRedoBlock     = errors.New("redo already requested for specified block")
	InvalidDataReceived    = errors.New("data receipt was acknowledged before all blocks were acknowledged")
)

type Sender struct {
	*protocol.Client

	reader        *block.Reader
	received      []bool
	receivedCount uint64

	metadata *Metadata
}

// NewSender creates a new Sender with Metadata.
// It reads data from a BlockReader which may not be mutated
// after it has been passed to this function.
func NewSender(metadata *Metadata, reader *block.Reader) *Sender {
	s := &Sender{
		metadata:      metadata,
		reader:        reader,
		received:      make([]bool, reader.BlockCount()),
		receivedCount: 0,
	}
	s.Client = protocol.NewClient(s.protocolDefinition())
	return s
}

// TODO
//  The sender type should buffer a couple of blocks.
//  That way a WriteMessage call won't block execution every time.
//  Messages from WriteMessage should not be buffered at all.

func (s *Sender) protocolDefinition() *protocol.Definition {
	return &protocol.Definition{
		Protocol:     Protocol{},
		InitialState: initialized,
		FinalState:   done,
		WriteMessage: Transitions{
			initialized: {
				Transition{
					To: inPrologue,
					Do: func(state *StateHandle) (message proto.Message, err error) {
						message = s.createHeader()
						return
					},
				},
			},
			inPrologue: {
				Transition{
					Ok: States{inPrologue, needAcknowledge},
					Do: func(state *StateHandle) (message proto.Message, err error) {
						if !s.reader.HasData() {
							panic("expected data but no data to write")
						}
						message, err = s.reader.NextBlock()
						if err != nil {
							return
						}
						if s.reader.IsDone() {
							state.Set(needAcknowledge)
						} else {
							state.Set(inPrologue)
						}
						return
					},
				},
			},
			inProgress: {
				Transition{
					Ok: States{inProgress, transferred},
					Do: func(state *StateHandle) (message proto.Message, err error) {
						if !s.reader.HasData() {
							panic("expected data but no data to write")
						}
						message, err = s.reader.NextBlock()
						if err != nil {
							// TODO: check if the automaton handles this case correctly.
							return
						}
						if s.reader.IsDone() {
							state.Set(transferred)
						} else {
							state.Set(inProgress)
						}
						return
					},
				},
			},
			transferred: {
				Transition{
					To: needDataReceived,
					// Do: protocol.WriteHandler(&DataSent{})
					Do: func(state *StateHandle) (message proto.Message, err error) {
						message = &DataSent{}
						return
					},
				},
			},
			sendAbort: {
				Transition{
					To: aborted,
					Do: func(state *StateHandle) (message proto.Message, err error) {
						message = &Abort{}
						return
					},
				},
			},
		},
		ReadMessage: Transitions{
			MessageTypeAcknowledge: {
				Transition{
					At: States{inPrologue},
					To: inProgress,
				},
				Transition{
					At: States{needAcknowledge},
					To: transferred,
				},
			},
			MessageTypeBlockReceived: {
				Transition{
					At: States{inProgress},
					Do: func(state *StateHandle, message proto.Message) (err error) {
						// TODO validate the message! could have an invalid index...
						b := message.(*BlockReceived)
						if s.received[b.Index] {
							err = DuplicateBlockReceived
							return
						}
						s.received[b.Index] = true
						s.receivedCount += 1
						if s.receivedCount > s.reader.BlockCount() {
							panic("sent more blocks than there should be available")
						}
						return
					},
				},
				Transition{
					At: States{transferred},
					Do: func(state *StateHandle, message proto.Message) (err error) {
						// TODO validate the message! could have an invalid index...
						b := message.(*BlockReceived)
						if s.received[b.Index] {
							err = DuplicateBlockReceived
							return
						}
						s.received[b.Index] = true
						s.receivedCount += 1
						if s.receivedCount > s.reader.BlockCount() {
							panic("sent more blocks than there should be available")
						}
						return
					},
				},
				Transition{
					At: States{needDataReceived},
					Do: func(state *StateHandle, message proto.Message) (err error) {
						// TODO validate the message! could have an invalid index...
						b := message.(*BlockReceived)
						if s.received[b.Index] {
							err = DuplicateBlockReceived
							return
						}
						s.received[b.Index] = true
						s.receivedCount += 1
						if s.receivedCount > s.reader.BlockCount() {
							panic("sent more blocks than there should be available")
						}
						return
					},
				},
			},
			MessageTypeRedoBlock: {
				Transition{
					At: States{inProgress},
					Do: func(state *StateHandle, message proto.Message) (err error) {
						// TODO validate the message! could have an invalid index...
						b := message.(*RedoBlock)
						if s.received[b.Index] {
							err = RedoForReceivedBlock
							return
						}
						if b.Index >= s.reader.NextBlockIndex() {
							err = RedoForUnwrittenBlock
							return
						}
						var queued bool
						queued, err = s.reader.RedoBlock(b.Index)
						if err != nil {
							return
						}
						if !queued {
							err = DuplicateRedoBlock
						}
						return
					},
				},
				Transition{
					At: States{transferred},
					To: inProgress,
					Do: func(state *StateHandle, message proto.Message) (err error) {
						// TODO validate the message! could have an invalid index...
						b := message.(*RedoBlock)
						if s.received[b.Index] {
							err = RedoForReceivedBlock
							return
						}
						if b.Index >= s.reader.NextBlockIndex() {
							err = RedoForUnwrittenBlock
							return
						}
						var queued bool
						queued, err = s.reader.RedoBlock(b.Index)
						if err != nil {
							return
						}
						if !queued {
							err = DuplicateRedoBlock
						}
						return
					},
				},
				Transition{
					At: States{needDataReceived},
					To: inProgress,
					Do: func(state *StateHandle, message proto.Message) (err error) {
						// TODO validate the message! could have an invalid index...
						b := message.(*RedoBlock)
						if s.received[b.Index] {
							err = RedoForReceivedBlock
							return
						}
						if b.Index >= s.reader.NextBlockIndex() {
							err = RedoForUnwrittenBlock
							return
						}
						var queued bool
						queued, err = s.reader.RedoBlock(b.Index)
						if err != nil {
							return
						}
						if !queued {
							err = DuplicateRedoBlock
						}
						return
					},
				},
			},
			MessageTypeDataReceived: {
				Transition{
					At: States{needDataReceived},
					To: done,
					Do: func(state *StateHandle, message proto.Message) (err error) {
						if s.receivedCount < s.reader.BlockCount() {
							err = InvalidDataReceived
						}
						return
					},
				},
			},
			MessageTypeAbort: {
				Transition{
					At: States{initialized, inPrologue, inProgress, needAcknowledge,
						transferred, needDataReceived, sendAbort},
					To: aborted,
				},
			},
		},
	}
}

// IsDone signals if the transfer has been completed or was aborted.
// The Sender instance may be discarded.
func (s *Sender) IsDone() bool {
	return s.State().Is(done) || s.IsAborted()
}

// IsAborted signals if this transfer has been aborted.
func (s *Sender) IsAborted() bool {
	return s.State().Is(aborted)
}

// Abort requests to abort the data transfer.
// WriteMessage must be called to properly communicate this state change to the Receiver.
// The transfer has been aborted once IsAborted returns true.
func (s *Sender) Abort() error {
	panic("not implemented")
	//s.state.Set(sendAbort)
	return s.reader.Close()
}

func (s *Sender) createHeader() *Header {
	return &Header{
		Metadata:  s.metadata,
		TotalSize: s.reader.TotalSize(),
		BlockSize: uint64(s.reader.BlockSize()),
	}
}
