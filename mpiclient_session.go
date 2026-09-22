package gos7

import (
	"errors"
	"io"
	"sync"
)

// ErrMPI2SessionUnusable means a prior exchange left the wire state uncertain.
var ErrMPI2SessionUnusable = errors.New("MPI2 byte session is unusable after an exchange failure")

// MPI2ByteSessionConfig defines an already connected and negotiated MPI2 link.
type MPI2ByteSessionConfig struct {
	Peer          byte
	Local         byte
	PDUSize       int
	Reference     uint16
	MessageNumber byte
}

// MPI2ByteSession serializes byte operations on a caller-owned MPI2 wire.
type MPI2ByteSession struct {
	mu            sync.Mutex
	wire          io.ReadWriter
	peer          byte
	local         byte
	pduSize       int
	reference     uint16
	messageNumber byte
	unusable      bool
}

// NewMPI2ByteSession attaches to a negotiated wire without performing I/O.
func NewMPI2ByteSession(wire io.ReadWriter, config MPI2ByteSessionConfig) (*MPI2ByteSession, error) {
	if wire == nil {
		return nil, io.ErrClosedPipe
	}
	if config.Local == 0 || config.PDUSize < 64 || config.PDUSize > 240 || config.Reference == 0 || config.MessageNumber == 0 {
		return nil, errors.New("invalid MPI2 byte session configuration")
	}
	return &MPI2ByteSession{
		wire: wire, peer: config.Peer, local: config.Local, pduSize: config.PDUSize,
		reference: config.Reference, messageNumber: config.MessageNumber,
	}, nil
}

// ReadBytes reads one complete byte range without exposing partial data.
func (session *MPI2ByteSession) ReadBytes(area byte, dbNumber, start, count int) ([]byte, error) {
	// The lock intentionally spans I/O: MPI2 frames and sequence numbers form
	// one ordered stream and must never interleave across callers.
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.unusable {
		return nil, ErrMPI2SessionUnusable
	}
	reference := session.referenceForRange(count, min(222, session.pduSize-18))
	request := MPI2ReadRangeRequest{
		Area: area, DBNumber: dbNumber, Start: start, Count: count,
		PDUSize: session.pduSize, Reference: reference, MessageNumber: session.messageNumber,
	}
	chunks, err := PlanMPI2ReadRange(request)
	if err != nil {
		return nil, err
	}
	data, err := ReadMPI2Range(session.wire, session.peer, session.local, request)
	if err != nil {
		session.unusable = true
		return nil, err
	}
	session.advance(len(chunks), reference)
	return data, nil
}

// WriteBytes writes a non-atomic byte range, preserving applied-prefix errors.
func (session *MPI2ByteSession) WriteBytes(area byte, dbNumber, start int, data []byte) error {
	// The lock intentionally spans I/O for the same reason as ReadBytes.
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.unusable {
		return ErrMPI2SessionUnusable
	}
	reference := session.referenceForRange(len(data), min(224, session.pduSize-28))
	request := MPI2WriteRangeRequest{
		Area: area, DBNumber: dbNumber, Start: start, Data: data,
		PDUSize: session.pduSize, Reference: reference, MessageNumber: session.messageNumber,
	}
	chunks, err := PlanMPI2WriteRange(request)
	if err != nil {
		return err
	}
	if err := WriteMPI2Range(session.wire, session.peer, session.local, request); err != nil {
		session.unusable = true
		return err
	}
	session.advance(len(chunks), reference)
	return nil
}

func (session *MPI2ByteSession) referenceForRange(length, chunkSize int) uint16 {
	if length <= 0 {
		return session.reference
	}
	needed := length / chunkSize
	if length%chunkSize != 0 {
		needed++
	}
	if needed > 65536-int(session.reference) {
		return 1
	}
	return session.reference
}

func (session *MPI2ByteSession) advance(chunks int, reference uint16) {
	nextReference := int(reference) + chunks
	if nextReference > 65535 {
		nextReference = 1
	}
	session.reference = uint16(nextReference)
	for i := 0; i < chunks; i++ {
		session.messageNumber = nextMPI2MessageNumber(session.messageNumber)
	}
}
