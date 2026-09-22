package gos7

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func mpi2ValidSetupReply() []byte {
	return []byte{0x32, 3, 0, 0, 0, 1, 0, 8, 0, 0, 0, 0, 0xf0, 0, 0, 1, 0, 1, 0, 0xc0}
}

func TestMPI2SetupCodec(t *testing.T) {
	t.Parallel()
	request := EncodeMPI2SetupRequest()
	want := []byte{0x32, 1, 0, 0, 0, 1, 0, 8, 0, 0, 0xf0, 0, 0, 1, 0, 1, 0, 0xf0}
	if !bytes.Equal(request, want) {
		t.Fatalf("request=%x want=%x", request, want)
	}
	request[0] = 0
	if second := EncodeMPI2SetupRequest(); second[0] != 0x32 {
		t.Fatal("setup request aliases template")
	}
	size, err := DecodeMPI2SetupResponse(mpi2ValidSetupReply())
	if err != nil || size != 192 {
		t.Fatalf("size=%d err=%v", size, err)
	}
}

func TestMPI2SetupRejectsMalformed(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		edit func([]byte) []byte
	}{
		{name: "short", edit: func(p []byte) []byte { return p[:19] }},
		{name: "long", edit: func(p []byte) []byte { return append(p, 0) }},
		{name: "protocol", edit: func(p []byte) []byte { p[0] = 0; return p }},
		{name: "type", edit: func(p []byte) []byte { p[1] = 2; return p }},
		{name: "reference", edit: func(p []byte) []byte { p[5] = 2; return p }},
		{name: "parameter length", edit: func(p []byte) []byte { p[7] = 7; return p }},
		{name: "data length", edit: func(p []byte) []byte { p[9] = 1; return p }},
		{name: "error class", edit: func(p []byte) []byte { p[10] = 1; return p }},
		{name: "error code", edit: func(p []byte) []byte { p[11] = 1; return p }},
		{name: "function", edit: func(p []byte) []byte { p[12] = 0; return p }},
		{name: "reserved", edit: func(p []byte) []byte { p[13] = 1; return p }},
		{name: "calling jobs", edit: func(p []byte) []byte { p[15] = 0; return p }},
		{name: "called jobs", edit: func(p []byte) []byte { p[17] = 0; return p }},
		{name: "too small", edit: func(p []byte) []byte { binary.BigEndian.PutUint16(p[18:20], 63); return p }},
		{name: "too large", edit: func(p []byte) []byte { binary.BigEndian.PutUint16(p[18:20], 241); return p }},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if size, err := DecodeMPI2SetupResponse(tt.edit(mpi2ValidSetupReply())); err == nil || size != 0 {
				t.Fatalf("accepted bad setup response: size=%d err=%v", size, err)
			}
		})
	}
}

func TestMPI2NegotiatePDU(t *testing.T) {
	t.Parallel()
	wire := &mpi2InitWire{reader: bytes.NewReader(mpi2TestPDUReplies(t))}
	size, err := NegotiateMPI2PDU(wire, 0x55, 3, 4)
	if err != nil || size != 192 || wire.reader.Len() != 0 {
		t.Fatalf("size=%d err=%v unread=%d", size, err, wire.reader.Len())
	}
	request, err := EncodeMPI2PDUEnvelope(0x55, 3, 4, EncodeMPI2SetupRequest())
	if err != nil {
		t.Fatal(err)
	}
	frame, err := EncodeMPI2Frame(request)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wire.writes.Bytes()[:1+len(frame)], append([]byte{mpi2STX}, frame...)) {
		t.Fatalf("wrong setup request: %x", wire.writes.Bytes())
	}
}

func TestMPI2NegotiatePDURejectsBadSetup(t *testing.T) {
	t.Parallel()
	response := mpi2ValidSetupReply()
	response[12] = 0
	ack, _ := EncodeMPI2MessageAck(0x55, 3, 4)
	ackFrame, _ := EncodeMPI2Frame(ack)
	responsePayload, _ := EncodeMPI2PDUEnvelope(0x55, 3, 9, response)
	responseFrame, _ := EncodeMPI2Frame(responsePayload)
	input := append([]byte{mpi2DLE, mpi2DLE, mpi2STX}, ackFrame...)
	input = append(input, mpi2STX)
	input = append(input, responseFrame...)
	input = append(input, mpi2DLE, mpi2DLE)
	wire := &mpi2InitWire{reader: bytes.NewReader(input)}
	if size, err := NegotiateMPI2PDU(wire, 0x55, 3, 4); err == nil || size != 0 {
		t.Fatalf("accepted rejected setup: size=%d err=%v", size, err)
	}
}
