package gos7

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func mpi2TestReadRequest() MPI2ReadRequest {
	return MPI2ReadRequest{Area: 0x84, DBNumber: 5, Start: 2, Count: 3, PDUSize: 192, Reference: 7}
}

func mpi2TestReadResponse() []byte {
	return []byte{0x32, 3, 0, 0, 0, 7, 0, 2, 0, 7, 0, 0, 4, 1, 0xff, 4, 0, 24, 0x11, 0x22, 0x33}
}

func TestMPI2ReadCodec(t *testing.T) {
	t.Parallel()
	request := mpi2TestReadRequest()
	pdu, err := EncodeMPI2ReadPDU(request)
	want := []byte{0x32, 1, 0, 0, 0, 7, 0, 14, 0, 0, 4, 1, 0x12, 0x0a, 0x10, 2, 0, 3, 0, 5, 0x84, 0, 0, 16}
	if err != nil || !bytes.Equal(pdu, want) {
		t.Fatalf("PDU=%x err=%v want=%x", pdu, err, want)
	}
	response := mpi2TestReadResponse()
	data, err := DecodeMPI2ReadResponse(response, request)
	if err != nil || !bytes.Equal(data, []byte{0x11, 0x22, 0x33}) {
		t.Fatalf("data=%x err=%v", data, err)
	}
	response[18] = 0
	if data[0] != 0x11 {
		t.Fatal("decoded data aliases response")
	}
	padded := append(bytes.Clone(mpi2TestReadResponse()), 0)
	padded[9]++
	if _, err := DecodeMPI2ReadResponse(padded, request); err != nil {
		t.Fatalf("zero-padded response: %v", err)
	}
}

func TestMPI2ReadRejectsInvalidRequest(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		edit func(*MPI2ReadRequest)
	}{
		{name: "area", edit: func(r *MPI2ReadRequest) { r.Area = 0 }},
		{name: "DB on marker", edit: func(r *MPI2ReadRequest) { r.Area = 0x83 }},
		{name: "negative DB", edit: func(r *MPI2ReadRequest) { r.DBNumber = -1 }},
		{name: "DB overflow", edit: func(r *MPI2ReadRequest) { r.DBNumber = 65536 }},
		{name: "negative start", edit: func(r *MPI2ReadRequest) { r.Start = -1 }},
		{name: "start overflow", edit: func(r *MPI2ReadRequest) { r.Start = 1 << 21 }},
		{name: "bit address overflow", edit: func(r *MPI2ReadRequest) { r.Start = 1 << 18 }},
		{name: "range overflow", edit: func(r *MPI2ReadRequest) { r.Start = (1 << 21) - 1; r.Count = 2 }},
		{name: "zero count", edit: func(r *MPI2ReadRequest) { r.Count = 0 }},
		{name: "over serial bound", edit: func(r *MPI2ReadRequest) { r.Count = 223 }},
		{name: "over negotiated PDU", edit: func(r *MPI2ReadRequest) { r.Count = 175 }},
		{name: "small PDU", edit: func(r *MPI2ReadRequest) { r.PDUSize = 63 }},
		{name: "large PDU", edit: func(r *MPI2ReadRequest) { r.PDUSize = 241 }},
		{name: "zero reference", edit: func(r *MPI2ReadRequest) { r.Reference = 0 }},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			request := mpi2TestReadRequest()
			tt.edit(&request)
			if pdu, err := EncodeMPI2ReadPDU(request); err == nil || pdu != nil {
				t.Fatalf("accepted invalid request %+v: PDU=%x err=%v", request, pdu, err)
			}
		})
	}
}

func TestMPI2ReadRejectsBadResponse(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		edit func([]byte) []byte
	}{
		{name: "short", edit: func(p []byte) []byte { return p[:17] }},
		{name: "protocol", edit: func(p []byte) []byte { p[0] = 0; return p }},
		{name: "type", edit: func(p []byte) []byte { p[1] = 1; return p }},
		{name: "reference", edit: func(p []byte) []byte { p[5] = 2; return p }},
		{name: "parameter length", edit: func(p []byte) []byte { p[7] = 1; return p }},
		{name: "data length", edit: func(p []byte) []byte { p[9] = 6; return p }},
		{name: "error", edit: func(p []byte) []byte { p[10] = 1; return p }},
		{name: "function", edit: func(p []byte) []byte { p[12] = 5; return p }},
		{name: "item count", edit: func(p []byte) []byte { p[13] = 2; return p }},
		{name: "return code", edit: func(p []byte) []byte { p[14] = 5; return p }},
		{name: "transport", edit: func(p []byte) []byte { p[15] = 3; return p }},
		{name: "bit length", edit: func(p []byte) []byte { p[17] = 16; return p }},
		{name: "trailing", edit: func(p []byte) []byte { return append(p, 0) }},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if data, err := DecodeMPI2ReadResponse(tt.edit(mpi2TestReadResponse()), mpi2TestReadRequest()); err == nil || data != nil {
				t.Fatalf("accepted malformed reply: data=%x err=%v", data, err)
			}
		})
	}
}

func TestMPI2ReadBytes(t *testing.T) {
	t.Parallel()
	request := mpi2TestReadRequest()
	ack, _ := EncodeMPI2MessageAck(0x55, 3, 4)
	ackFrame, _ := EncodeMPI2Frame(ack)
	responsePayload, _ := EncodeMPI2PDUEnvelope(0x55, 3, 9, mpi2TestReadResponse())
	responseFrame, _ := EncodeMPI2Frame(responsePayload)
	input := append([]byte{mpi2DLE, mpi2DLE, mpi2STX}, ackFrame...)
	input = append(input, mpi2STX)
	input = append(input, responseFrame...)
	input = append(input, mpi2DLE, mpi2DLE)
	wire := &mpi2InitWire{reader: bytes.NewReader(input)}
	data, err := ReadMPI2Bytes(wire, 0x55, 3, 4, request)
	if err != nil || !bytes.Equal(data, []byte{0x11, 0x22, 0x33}) || wire.reader.Len() != 0 {
		t.Fatalf("data=%x err=%v unread=%d", data, err, wire.reader.Len())
	}
	pdu, _ := EncodeMPI2ReadPDU(request)
	requestPayload, _ := EncodeMPI2PDUEnvelope(0x55, 3, 4, pdu)
	requestFrame, _ := EncodeMPI2Frame(requestPayload)
	if !bytes.Equal(wire.writes.Bytes()[:1+len(requestFrame)], append([]byte{mpi2STX}, requestFrame...)) {
		t.Fatalf("wrong read request: %x", wire.writes.Bytes())
	}
	bad := mpi2TestReadResponse()
	binary.BigEndian.PutUint16(bad[16:18], 16)
	if data, err := DecodeMPI2ReadResponse(bad, request); err == nil || data != nil {
		t.Fatalf("accepted wrong bit length: %x %v", data, err)
	}
}

func TestMPI2ReadBytesRejectsBeforeIOAndNoPartialData(t *testing.T) {
	t.Parallel()
	request := mpi2TestReadRequest()
	request.Start = 1 << 18
	wire := &mpi2InitWire{reader: bytes.NewReader(nil)}
	if data, err := ReadMPI2Bytes(wire, 0x55, 3, 4, request); err == nil || data != nil || wire.writes.Len() != 0 {
		t.Fatalf("invalid request caused I/O: data=%x err=%v writes=%x", data, err, wire.writes.Bytes())
	}
	request = mpi2TestReadRequest()
	ack, _ := EncodeMPI2MessageAck(0x55, 3, 4)
	ackFrame, _ := EncodeMPI2Frame(ack)
	bad := mpi2TestReadResponse()
	bad[14] = 5
	responsePayload, _ := EncodeMPI2PDUEnvelope(0x55, 3, 9, bad)
	responseFrame, _ := EncodeMPI2Frame(responsePayload)
	input := append([]byte{mpi2DLE, mpi2DLE, mpi2STX}, ackFrame...)
	input = append(input, mpi2STX)
	input = append(input, responseFrame...)
	input = append(input, mpi2DLE, mpi2DLE)
	wire = &mpi2InitWire{reader: bytes.NewReader(input)}
	if data, err := ReadMPI2Bytes(wire, 0x55, 3, 4, request); err == nil || data != nil {
		t.Fatalf("returned partial result: data=%x err=%v", data, err)
	}
}
