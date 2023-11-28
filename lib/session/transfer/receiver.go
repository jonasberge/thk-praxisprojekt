package transfer

import (
	"errors"
	"fmt"
	"google.golang.org/protobuf/proto"
	. "projekt/core/lib/automaton"
	"projekt/core/lib/protocol"
	"projekt/core/lib/session/transfer/block"
)

/*

TODO:
 Add ID to payload
 The ID is used to signal:
  - I already have this data, transmission is not necessary (receiver side)
    The receiver will still show the data in the application as recently received
    It just uses the content that was previously received
  - I want to continue sending this data because transmission was interrupted (sender side)
    The receiver needs to answer with the block index until which they received all data.
    It's possible to say that they already have all the data.

*/

const (
	sendAcknowledge = iota + _firstState
	needBlock
	handleBlock
	needDataSent
	sendDataReceived
)

type Receiver struct {
	*protocol.Client

	writerFactory  WriterFactory
	writer         *block.Writer
	nextBlockIndex uint64

	lastBlock  *Block
	redoBlocks []uint64

	header *Header
}

type WriterFactory = func(metadata *Metadata, size uint64) (block.WriteSeekCloser, error)

// NewReceiver creates a new Receiver.
// It writes received data to a BlockWriter which may not be mutated
// after it has been passed to this function.
func NewReceiver(writerFactory WriterFactory) *Receiver {
	s := &Receiver{
		writerFactory:  writerFactory,
		writer:         nil, // on writerFactory
		nextBlockIndex: 0,
		lastBlock:      nil, // on NeedBlock
		redoBlocks:     nil, // on NeedBlock
		header:         nil, // on Header
	}
	s.Client = protocol.NewClient(s.protocolDefinition())
	return s
}

func (s *Receiver) protocolDefinition() *protocol.Definition {
	return &protocol.Definition{
		Protocol:     Protocol{},
		WriteFirst:   true,
		InitialState: initialized,
		FinalState:   done,
		WriteMessage: Transitions{
			sendAcknowledge: {
				Transition{
					To: needBlock,
					Do: func(state *StateHandle) (message proto.Message, err error) {
						message = &Acknowledge{}
						return
					},
				},
			},
			handleBlock: {
				Transition{
					Ok: States{handleBlock, needBlock, needDataSent},
					Do: func(state *StateHandle) (message proto.Message, err error) {
						if s.lastBlock != nil {
							message = &BlockReceived{Index: s.lastBlock.Index}
							s.lastBlock = nil
						} else if s.redoBlocks != nil {
							redoIndex := s.redoBlocks[0]
							message = &RedoBlock{Index: redoIndex}
							if len(s.redoBlocks) == 1 {
								s.redoBlocks = nil
							} else {
								s.redoBlocks = s.redoBlocks[1:]
							}
						}
						if s.redoBlocks != nil {
							state.Set(handleBlock)
						} else if s.writer.IsDone() {
							state.Set(needDataSent)
						} else {
							state.Set(needBlock)
						}
						return
					},
				},
			},
			sendDataReceived: {
				Transition{
					To: done,
					Do: func(state *StateHandle) (message proto.Message, err error) {
						message = &DataReceived{}
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
			MessageTypeHeader: {
				Transition{
					At: States{initialized},
					To: sendAcknowledge,
					Do: func(state *StateHandle, message proto.Message) (err error) {
						header := message.(*Header)
						if header.Metadata == nil {
							return errors.New("header does not contain metadata")
						}
						if header.BlockSize == 0 {
							return errors.New("header contains an illegal block size of 0")
						}
						writer, err := s.writerFactory(header.Metadata, header.TotalSize)
						if err != nil {
							return
						}
						s.writer = block.NewWriter(writer, header.TotalSize, block.Size(header.BlockSize))
						s.header = header
						return
					},
				},
			},
			MessageTypeBlock: {
				Transition{
					At: States{needBlock},
					To: handleBlock,
					// TODO: use proto.Message instead of session.Message
					//  the type is not really useful to us, since we are casting anyway
					Do: func(state *StateHandle, message proto.Message) (err error) {
						b := message.(*Block)
						//
						// TODO Make WriteBlock concurrent and buffered
						//  Do not return an error with this function
						//  Instead handle errors in the background
						//   and when it's unrecoverable abort the transmission
						//  Otherwise send DataPersisted once all data has been written.
						//  We might need a Flush method on the WriteCloseSeeker (WriteCloseFlushSeeker?)
						//   in order to be sure that the data was persisted for good.
						//
						// This would make all calls in the protocol non-blocking.
						// We also aren't bottle-necked by writing-on-data where we skip buffering.
						//
						err = s.writer.WriteBlock(b)
						if err == block.BadBlockChecksum {
							fmt.Println("BAD BLOCK CHECKSUM")
							s.redoBlocks = []uint64{b.Index}
							err = nil
							return
						}
						if err != nil {
							return err
						}
						if b.Index > s.nextBlockIndex {
							// I wonder when this will happen realistically...
							s.redoBlocks = make([]uint64, 0, b.Index-s.nextBlockIndex)
							for i := s.nextBlockIndex; i < b.Index; i++ {
								s.redoBlocks = append(s.redoBlocks, i)
							}
						} else if b.Index < s.nextBlockIndex {
							panic("block already written")
						} else {
							s.nextBlockIndex += 1
						}
						s.lastBlock = b
						return
					},
				},
			},
			MessageTypeDataSent: {
				Transition{
					At: States{needDataSent},
					To: sendDataReceived,
				},
			},
		},
	}
}

// IsDone signals if the transfer has been completed or was aborted.
// The Receiver instance may be discarded.
func (s *Receiver) IsDone() bool {
	return s.State().Is(done) || s.IsAborted()
}

// IsAborted signals if this transfer has been aborted.
func (s *Receiver) IsAborted() bool {
	return s.State().Is(aborted)
}

// Abort requests to abort the data transfer.
// WriteMessage must be called to properly communicate this state change to the Sender.
// The transfer has been aborted once IsAborted returns true.
func (s *Receiver) Abort() error {
	panic("not implemented")
	//s.state.Set(sendAbort)
	return s.writer.Close()
}
