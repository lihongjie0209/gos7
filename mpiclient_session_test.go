package gos7

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"
)

func mpi2TestByteSessionConfig() MPI2ByteSessionConfig {
	return MPI2ByteSessionConfig{Peer: 0x55, Local: 3, PDUSize: 64, Reference: 2, MessageNumber: 5}
}

func TestMPI2ByteSessionReadThenWriteAdvancesNumbers(t *testing.T) {
	t.Parallel()
	input := append(mpi2RangeReply(t, 5, mpi2RangeReadReply(2, []byte{0x11, 0x22})), mpi2RangeReply(t, 6, mpi2RangeWriteReply(3))...)
	wire := &mpi2InitWire{reader: bytes.NewReader(input)}
	session, err := NewMPI2ByteSession(wire, mpi2TestByteSessionConfig())
	if err != nil {
		t.Fatal(err)
	}
	data, err := session.ReadBytes(0x84, 5, 2, 2)
	if err != nil || !bytes.Equal(data, []byte{0x11, 0x22}) {
		t.Fatalf("read=%x err=%v", data, err)
	}
	if err := session.WriteBytes(0x84, 5, 4, []byte{0x33}); err != nil || wire.reader.Len() != 0 {
		t.Fatalf("write err=%v unread=%d", err, wire.reader.Len())
	}
	for _, tt := range []struct {
		number    byte
		reference uint16
		read      bool
	}{
		{number: 5, reference: 2, read: true},
		{number: 6, reference: 3, read: false},
	} {
		var pdu []byte
		if tt.read {
			pdu, _ = EncodeMPI2ReadPDU(MPI2ReadRequest{Area: 0x84, DBNumber: 5, Start: 2, Count: 2, PDUSize: 64, Reference: tt.reference})
		} else {
			pdu, _ = EncodeMPI2WritePDU(MPI2WriteRequest{Area: 0x84, DBNumber: 5, Start: 4, Data: []byte{0x33}, PDUSize: 64, Reference: tt.reference})
		}
		payload, _ := EncodeMPI2PDUEnvelope(0x55, 3, tt.number, pdu)
		frame, _ := EncodeMPI2Frame(payload)
		if !bytes.Contains(wire.writes.Bytes(), frame) {
			t.Fatalf("missing request number %d ref %d in %x", tt.number, tt.reference, wire.writes.Bytes())
		}
	}
}

func TestMPI2ByteSessionWrapsBetweenOperations(t *testing.T) {
	t.Parallel()
	config := mpi2TestByteSessionConfig()
	config.Reference = 65535
	config.MessageNumber = 255
	input := append(mpi2RangeReply(t, 255, mpi2RangeReadReply(65535, []byte{1})), mpi2RangeReply(t, 1, mpi2RangeReadReply(1, []byte{2}))...)
	wire := &mpi2InitWire{reader: bytes.NewReader(input)}
	session, err := NewMPI2ByteSession(wire, config)
	if err != nil {
		t.Fatal(err)
	}
	first, err := session.ReadBytes(0x84, 0, 0, 1)
	if err != nil || !bytes.Equal(first, []byte{1}) {
		t.Fatalf("first=%x err=%v", first, err)
	}
	second, err := session.ReadBytes(0x84, 0, 1, 1)
	if err != nil || !bytes.Equal(second, []byte{2}) || wire.reader.Len() != 0 {
		t.Fatalf("second=%x err=%v unread=%d", second, err, wire.reader.Len())
	}
}

func TestMPI2ByteSessionAdvancesByChunkCount(t *testing.T) {
	t.Parallel()
	first := bytes.Repeat([]byte{0x11}, 46)
	input := append(mpi2RangeReply(t, 5, mpi2RangeReadReply(2, first)), mpi2RangeReply(t, 6, mpi2RangeReadReply(3, []byte{0x22}))...)
	input = append(input, mpi2RangeReply(t, 7, mpi2RangeWriteReply(4))...)
	wire := &mpi2InitWire{reader: bytes.NewReader(input)}
	session, err := NewMPI2ByteSession(wire, mpi2TestByteSessionConfig())
	if err != nil {
		t.Fatal(err)
	}
	data, err := session.ReadBytes(0x84, 5, 2, 47)
	if err != nil || !bytes.Equal(data, append(bytes.Clone(first), 0x22)) {
		t.Fatalf("chunked read=%x err=%v", data, err)
	}
	if err := session.WriteBytes(0x84, 5, 49, []byte{0x33}); err != nil || wire.reader.Len() != 0 {
		t.Fatalf("following write err=%v unread=%d", err, wire.reader.Len())
	}
}

func TestMPI2ByteSessionRestartsReferenceBeforeMultiChunkRange(t *testing.T) {
	t.Parallel()
	config := mpi2TestByteSessionConfig()
	config.Reference = 65535
	first := bytes.Repeat([]byte{0x11}, 46)
	input := append(mpi2RangeReply(t, 5, mpi2RangeReadReply(1, first)), mpi2RangeReply(t, 6, mpi2RangeReadReply(2, []byte{0x22}))...)
	wire := &mpi2InitWire{reader: bytes.NewReader(input)}
	session, err := NewMPI2ByteSession(wire, config)
	if err != nil {
		t.Fatal(err)
	}
	data, err := session.ReadBytes(0x84, 5, 0, 47)
	if err != nil || !bytes.Equal(data, append(bytes.Clone(first), 0x22)) || wire.reader.Len() != 0 {
		t.Fatalf("wrapped read=%x err=%v unread=%d", data, err, wire.reader.Len())
	}
}

func TestMPI2ByteSessionInvalidCallDoesNotConsumeNumbers(t *testing.T) {
	t.Parallel()
	input := mpi2RangeReply(t, 5, mpi2RangeReadReply(2, []byte{0x11}))
	wire := &mpi2InitWire{reader: bytes.NewReader(input)}
	session, err := NewMPI2ByteSession(wire, mpi2TestByteSessionConfig())
	if err != nil {
		t.Fatal(err)
	}
	if data, err := session.ReadBytes(0x84, 5, 1<<18, 1); err == nil || data != nil || wire.writes.Len() != 0 {
		t.Fatalf("invalid call: data=%x err=%v writes=%x", data, err, wire.writes.Bytes())
	}
	if data, err := session.ReadBytes(0x84, 5, 0, 1); err != nil || !bytes.Equal(data, []byte{0x11}) {
		t.Fatalf("valid call after invalid: data=%x err=%v", data, err)
	}
}

func TestMPI2ByteSessionPoisonsAfterWireError(t *testing.T) {
	t.Parallel()
	bad := mpi2RangeReadReply(2, []byte{0x11})
	bad[14] = 5
	wire := &mpi2InitWire{reader: bytes.NewReader(mpi2RangeReply(t, 5, bad))}
	session, err := NewMPI2ByteSession(wire, mpi2TestByteSessionConfig())
	if err != nil {
		t.Fatal(err)
	}
	if data, err := session.ReadBytes(0x84, 5, 0, 1); err == nil || data != nil {
		t.Fatalf("bad reply: data=%x err=%v", data, err)
	}
	written := wire.writes.Len()
	if err := session.WriteBytes(0x84, 5, 0, []byte{1}); !errors.Is(err, ErrMPI2SessionUnusable) || wire.writes.Len() != written {
		t.Fatalf("poisoned session: err=%v writes=%x", err, wire.writes.Bytes())
	}
}

func TestMPI2ByteSessionSerializesConcurrentReads(t *testing.T) {
	t.Parallel()
	input := append(mpi2RangeReply(t, 5, mpi2RangeReadReply(2, []byte{1})), mpi2RangeReply(t, 6, mpi2RangeReadReply(3, []byte{2}))...)
	wire := &mpi2InitWire{reader: bytes.NewReader(input)}
	session, err := NewMPI2ByteSession(wire, mpi2TestByteSessionConfig())
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan byte, 2)
	errorsSeen := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := session.ReadBytes(0x84, 5, 0, 1)
			if err != nil {
				errorsSeen <- err
				return
			}
			results <- data[0]
		}()
	}
	wg.Wait()
	close(results)
	close(errorsSeen)
	for err := range errorsSeen {
		t.Fatal(err)
	}
	seen := map[byte]bool{}
	for result := range results {
		seen[result] = true
	}
	if !seen[1] || !seen[2] || wire.reader.Len() != 0 {
		t.Fatalf("concurrent results=%v unread=%d", seen, wire.reader.Len())
	}
}

func TestMPI2ByteSessionConstructorRejectsInvalid(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		edit func(*MPI2ByteSessionConfig)
	}{
		{name: "local", edit: func(c *MPI2ByteSessionConfig) { c.Local = 0 }},
		{name: "small PDU", edit: func(c *MPI2ByteSessionConfig) { c.PDUSize = 63 }},
		{name: "large PDU", edit: func(c *MPI2ByteSessionConfig) { c.PDUSize = 241 }},
		{name: "reference", edit: func(c *MPI2ByteSessionConfig) { c.Reference = 0 }},
		{name: "message", edit: func(c *MPI2ByteSessionConfig) { c.MessageNumber = 0 }},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			config := mpi2TestByteSessionConfig()
			tt.edit(&config)
			wire := &mpi2InitWire{reader: bytes.NewReader(nil)}
			if session, err := NewMPI2ByteSession(wire, config); err == nil || session != nil || wire.writes.Len() != 0 {
				t.Fatalf("invalid constructor: session=%v err=%v writes=%x", session, err, wire.writes.Bytes())
			}
		})
	}
	if session, err := NewMPI2ByteSession(nil, mpi2TestByteSessionConfig()); !errors.Is(err, io.ErrClosedPipe) || session != nil {
		t.Fatalf("nil wire: session=%v err=%v", session, err)
	}
}
