package gos7

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func mpi2RangeReply(t *testing.T, message byte, pdu []byte) []byte {
	t.Helper()
	ack, err := EncodeMPI2MessageAck(0x55, 3, message)
	if err != nil {
		t.Fatal(err)
	}
	ackFrame, err := EncodeMPI2Frame(ack)
	if err != nil {
		t.Fatal(err)
	}
	response, err := EncodeMPI2PDUEnvelope(0x55, 3, message+10, pdu)
	if err != nil {
		t.Fatal(err)
	}
	responseFrame, err := EncodeMPI2Frame(response)
	if err != nil {
		t.Fatal(err)
	}
	input := append([]byte{mpi2DLE, mpi2DLE, mpi2STX}, ackFrame...)
	input = append(input, mpi2STX)
	input = append(input, responseFrame...)
	return append(input, mpi2DLE, mpi2DLE)
}

func mpi2RangeReadReply(reference uint16, data []byte) []byte {
	pdu := []byte{0x32, 3, 0, 0, 0, 0, 0, 2, 0, 0, 0, 0, 4, 1, 0xff, 4, 0, 0}
	binary.BigEndian.PutUint16(pdu[4:6], reference)
	binary.BigEndian.PutUint16(pdu[8:10], uint16(len(data)+4))
	binary.BigEndian.PutUint16(pdu[16:18], uint16(len(data)*8))
	return append(pdu, data...)
}

func mpi2RangeWriteReply(reference uint16) []byte {
	pdu := mpi2TestWriteResponse()
	binary.BigEndian.PutUint16(pdu[4:6], reference)
	return pdu
}

func TestMPI2RangePlansChunksAndWrapsMessageNumber(t *testing.T) {
	t.Parallel()
	read := MPI2ReadRangeRequest{Area: 0x84, DBNumber: 5, Start: 2, Count: 47, PDUSize: 64, Reference: 2, MessageNumber: 255}
	reads, err := PlanMPI2ReadRange(read)
	if err != nil || len(reads) != 2 || reads[0].Count != 46 || reads[1].Count != 1 || reads[1].Start != 48 || reads[1].Reference != 3 || reads[0].MessageNumber != 255 || reads[1].MessageNumber != 1 {
		t.Fatalf("read plan=%+v err=%v", reads, err)
	}
	write := MPI2WriteRangeRequest{Area: 0x84, DBNumber: 5, Start: 2, Data: make([]byte, 37), PDUSize: 64, Reference: 2, MessageNumber: 255}
	writes, err := PlanMPI2WriteRange(write)
	if err != nil || len(writes) != 2 || len(writes[0].Data) != 36 || len(writes[1].Data) != 1 || writes[1].Start != 38 || writes[1].Reference != 3 || writes[1].MessageNumber != 1 {
		t.Fatalf("write plan=%+v err=%v", writes, err)
	}
}

func TestMPI2RangeRejectsInvalidBeforeIO(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		read MPI2ReadRangeRequest
	}{
		{name: "zero count", read: MPI2ReadRangeRequest{Area: 0x84, PDUSize: 64, Reference: 2, MessageNumber: 5}},
		{name: "address overflow", read: MPI2ReadRangeRequest{Area: 0x84, Start: (1 << 18) - 1, Count: 2, PDUSize: 64, Reference: 2, MessageNumber: 5}},
		{name: "reference overflow", read: MPI2ReadRangeRequest{Area: 0x84, Count: 47, PDUSize: 64, Reference: 65535, MessageNumber: 5}},
		{name: "zero message", read: MPI2ReadRangeRequest{Area: 0x84, Count: 1, PDUSize: 64, Reference: 2}},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wire := &mpi2InitWire{reader: bytes.NewReader(nil)}
			if data, err := ReadMPI2Range(wire, 0x55, 3, tt.read); err == nil || data != nil || wire.writes.Len() != 0 {
				t.Fatalf("invalid read caused I/O: data=%x err=%v writes=%x", data, err, wire.writes.Bytes())
			}
		})
	}
	write := MPI2WriteRangeRequest{Area: 0x84, Start: (1 << 18) - 1, Data: []byte{1, 2}, PDUSize: 64, Reference: 2, MessageNumber: 5}
	wire := &mpi2InitWire{reader: bytes.NewReader(nil)}
	if err := WriteMPI2Range(wire, 0x55, 3, write); err == nil || wire.writes.Len() != 0 {
		t.Fatalf("invalid write caused I/O: err=%v writes=%x", err, wire.writes.Bytes())
	}
}

func TestMPI2ReadRangeReturnsOnlyCompleteData(t *testing.T) {
	t.Parallel()
	request := MPI2ReadRangeRequest{Area: 0x84, DBNumber: 5, Start: 2, Count: 47, PDUSize: 64, Reference: 2, MessageNumber: 5}
	first := bytes.Repeat([]byte{0x11}, 46)
	input := append(mpi2RangeReply(t, 5, mpi2RangeReadReply(2, first)), mpi2RangeReply(t, 6, mpi2RangeReadReply(3, []byte{0x22}))...)
	wire := &mpi2InitWire{reader: bytes.NewReader(input)}
	data, err := ReadMPI2Range(wire, 0x55, 3, request)
	if err != nil || !bytes.Equal(data, append(bytes.Clone(first), 0x22)) || wire.reader.Len() != 0 {
		t.Fatalf("data=%x err=%v unread=%d", data, err, wire.reader.Len())
	}
	bad := mpi2RangeReadReply(3, []byte{0x22})
	bad[14] = 5
	input = append(mpi2RangeReply(t, 5, mpi2RangeReadReply(2, first)), mpi2RangeReply(t, 6, bad)...)
	wire = &mpi2InitWire{reader: bytes.NewReader(input)}
	if data, err := ReadMPI2Range(wire, 0x55, 3, request); err == nil || data != nil {
		t.Fatalf("partial read returned: data=%x err=%v", data, err)
	}
}

func TestMPI2WriteRangeReportsAppliedPrefix(t *testing.T) {
	t.Parallel()
	request := MPI2WriteRangeRequest{Area: 0x84, DBNumber: 5, Start: 2, Data: bytes.Repeat([]byte{0x11}, 37), PDUSize: 64, Reference: 2, MessageNumber: 5}
	input := append(mpi2RangeReply(t, 5, mpi2RangeWriteReply(2)), mpi2RangeReply(t, 6, mpi2RangeWriteReply(3))...)
	wire := &mpi2InitWire{reader: bytes.NewReader(input)}
	if err := WriteMPI2Range(wire, 0x55, 3, request); err != nil || wire.reader.Len() != 0 {
		t.Fatalf("write err=%v unread=%d", err, wire.reader.Len())
	}
	bad := mpi2RangeWriteReply(3)
	bad[14] = 5
	input = append(mpi2RangeReply(t, 5, mpi2RangeWriteReply(2)), mpi2RangeReply(t, 6, bad)...)
	wire = &mpi2InitWire{reader: bytes.NewReader(input)}
	if err := WriteMPI2Range(wire, 0x55, 3, request); err == nil || !strings.Contains(err.Error(), "36 bytes") {
		t.Fatalf("missing applied-prefix information: %v", err)
	}
}
