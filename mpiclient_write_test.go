package gos7

import (
	"bytes"
	"testing"
)

func mpi2TestWriteRequest() MPI2WriteRequest {
	return MPI2WriteRequest{Area: 0x84, DBNumber: 5, Start: 2, Data: []byte{0x11, 0x22, 0x33}, PDUSize: 192, Reference: 7}
}

func mpi2TestWriteResponse() []byte {
	return []byte{0x32, 3, 0, 0, 0, 7, 0, 2, 0, 1, 0, 0, 5, 1, 0xff}
}

func TestMPI2WriteCodec(t *testing.T) {
	t.Parallel()
	request := mpi2TestWriteRequest()
	pdu, err := EncodeMPI2WritePDU(request)
	want := []byte{0x32, 1, 0, 0, 0, 7, 0, 14, 0, 7, 5, 1, 0x12, 0x0a, 0x10, 2, 0, 3, 0, 5, 0x84, 0, 0, 16, 0, 4, 0, 24, 0x11, 0x22, 0x33}
	if err != nil || !bytes.Equal(pdu, want) {
		t.Fatalf("PDU=%x err=%v want=%x", pdu, err, want)
	}
	request.Data[0] = 0
	if pdu[28] != 0x11 {
		t.Fatal("encoded PDU aliases caller data")
	}
	if err := DecodeMPI2WriteResponse(mpi2TestWriteResponse(), mpi2TestWriteRequest()); err != nil {
		t.Fatal(err)
	}
}

func TestMPI2WriteRejectsInvalidRequest(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		edit func(*MPI2WriteRequest)
	}{
		{name: "area", edit: func(r *MPI2WriteRequest) { r.Area = 0 }},
		{name: "DB on marker", edit: func(r *MPI2WriteRequest) { r.Area = 0x83 }},
		{name: "negative DB", edit: func(r *MPI2WriteRequest) { r.DBNumber = -1 }},
		{name: "DB overflow", edit: func(r *MPI2WriteRequest) { r.DBNumber = 65536 }},
		{name: "negative start", edit: func(r *MPI2WriteRequest) { r.Start = -1 }},
		{name: "bit address overflow", edit: func(r *MPI2WriteRequest) { r.Start = 1 << 18 }},
		{name: "range overflow", edit: func(r *MPI2WriteRequest) { r.Start = (1 << 18) - 1; r.Data = []byte{1, 2} }},
		{name: "empty data", edit: func(r *MPI2WriteRequest) { r.Data = nil }},
		{name: "over serial bound", edit: func(r *MPI2WriteRequest) { r.Data = make([]byte, 225) }},
		{name: "over negotiated PDU", edit: func(r *MPI2WriteRequest) { r.Data = make([]byte, 165) }},
		{name: "small PDU", edit: func(r *MPI2WriteRequest) { r.PDUSize = 63 }},
		{name: "large PDU", edit: func(r *MPI2WriteRequest) { r.PDUSize = 241 }},
		{name: "zero reference", edit: func(r *MPI2WriteRequest) { r.Reference = 0 }},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			request := mpi2TestWriteRequest()
			tt.edit(&request)
			if pdu, err := EncodeMPI2WritePDU(request); err == nil || pdu != nil {
				t.Fatalf("accepted invalid request %+v: PDU=%x err=%v", request, pdu, err)
			}
		})
	}
}

func TestMPI2WriteRejectsBadResponse(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		edit func([]byte) []byte
	}{
		{name: "short", edit: func(p []byte) []byte { return p[:14] }},
		{name: "long", edit: func(p []byte) []byte { return append(p, 0) }},
		{name: "protocol", edit: func(p []byte) []byte { p[0] = 0; return p }},
		{name: "type", edit: func(p []byte) []byte { p[1] = 1; return p }},
		{name: "reserved", edit: func(p []byte) []byte { p[2] = 1; return p }},
		{name: "reference", edit: func(p []byte) []byte { p[5] = 2; return p }},
		{name: "parameter length", edit: func(p []byte) []byte { p[7] = 1; return p }},
		{name: "data length", edit: func(p []byte) []byte { p[9] = 2; return p }},
		{name: "error", edit: func(p []byte) []byte { p[10] = 1; return p }},
		{name: "function", edit: func(p []byte) []byte { p[12] = 4; return p }},
		{name: "item count", edit: func(p []byte) []byte { p[13] = 2; return p }},
		{name: "status", edit: func(p []byte) []byte { p[14] = 5; return p }},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := DecodeMPI2WriteResponse(tt.edit(mpi2TestWriteResponse()), mpi2TestWriteRequest()); err == nil {
				t.Fatal("accepted malformed write response")
			}
		})
	}
}

func TestMPI2WriteBytes(t *testing.T) {
	t.Parallel()
	ack, _ := EncodeMPI2MessageAck(0x55, 3, 4)
	ackFrame, _ := EncodeMPI2Frame(ack)
	responsePayload, _ := EncodeMPI2PDUEnvelope(0x55, 3, 9, mpi2TestWriteResponse())
	responseFrame, _ := EncodeMPI2Frame(responsePayload)
	input := append([]byte{mpi2DLE, mpi2DLE, mpi2STX}, ackFrame...)
	input = append(input, mpi2STX)
	input = append(input, responseFrame...)
	input = append(input, mpi2DLE, mpi2DLE)
	wire := &mpi2InitWire{reader: bytes.NewReader(input)}
	if err := WriteMPI2Bytes(wire, 0x55, 3, 4, mpi2TestWriteRequest()); err != nil || wire.reader.Len() != 0 {
		t.Fatalf("write err=%v unread=%d", err, wire.reader.Len())
	}
	pdu, _ := EncodeMPI2WritePDU(mpi2TestWriteRequest())
	requestPayload, _ := EncodeMPI2PDUEnvelope(0x55, 3, 4, pdu)
	requestFrame, _ := EncodeMPI2Frame(requestPayload)
	if !bytes.Equal(wire.writes.Bytes()[:1+len(requestFrame)], append([]byte{mpi2STX}, requestFrame...)) {
		t.Fatalf("wrong write request: %x", wire.writes.Bytes())
	}
}

func TestMPI2WriteBytesRejectsBeforeIO(t *testing.T) {
	t.Parallel()
	request := mpi2TestWriteRequest()
	request.Start = 1 << 18
	wire := &mpi2InitWire{reader: bytes.NewReader(nil)}
	if err := WriteMPI2Bytes(wire, 0x55, 3, 4, request); err == nil || wire.writes.Len() != 0 {
		t.Fatalf("invalid request caused I/O: err=%v writes=%x", err, wire.writes.Bytes())
	}
}
