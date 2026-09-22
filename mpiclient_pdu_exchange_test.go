package gos7

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func mpi2TestSetupRequest() []byte {
	return []byte{0x32, 1, 0, 0, 0, 1, 0, 8, 0, 0, 0xf0, 0, 0, 1, 0, 1, 0, 0xc0}
}

func mpi2TestSetupResponse() []byte {
	return []byte{0x32, 3, 0, 0, 0, 1, 0, 8, 0, 0, 0, 0, 0xf0, 0, 0, 1, 0, 1, 0, 0xc0}
}

func mpi2TestPDUReplies(t *testing.T) []byte {
	t.Helper()
	ack, err := EncodeMPI2MessageAck(0x55, 3, 4)
	if err != nil {
		t.Fatal(err)
	}
	ackFrame, err := EncodeMPI2Frame(ack)
	if err != nil {
		t.Fatal(err)
	}
	response, err := EncodeMPI2PDUEnvelope(0x55, 3, 9, mpi2TestSetupResponse())
	if err != nil {
		t.Fatal(err)
	}
	responseFrame, err := EncodeMPI2Frame(response)
	if err != nil {
		t.Fatal(err)
	}
	replies := append([]byte{mpi2DLE, mpi2DLE, mpi2STX}, ackFrame...)
	replies = append(replies, mpi2STX)
	replies = append(replies, responseFrame...)
	return append(replies, mpi2DLE, mpi2DLE)
}

func TestMPI2S7PDUExchange(t *testing.T) {
	t.Parallel()
	wire := &mpi2InitWire{reader: bytes.NewReader(mpi2TestPDUReplies(t))}
	response, err := ExchangeMPI2S7PDU(wire, 0x55, 3, 4, mpi2TestSetupRequest())
	if err != nil || !bytes.Equal(response, mpi2TestSetupResponse()) || wire.reader.Len() != 0 {
		t.Fatalf("response=%x err=%v unread=%d", response, err, wire.reader.Len())
	}
	request, err := EncodeMPI2PDUEnvelope(0x55, 3, 4, mpi2TestSetupRequest())
	if err != nil {
		t.Fatal(err)
	}
	requestFrame, err := EncodeMPI2Frame(request)
	if err != nil {
		t.Fatal(err)
	}
	ack, err := EncodeMPI2MessageAck(0x55, 3, 9)
	if err != nil {
		t.Fatal(err)
	}
	ackFrame, err := EncodeMPI2Frame(ack)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]byte{mpi2STX}, requestFrame...)
	want = append(want, mpi2DLE, mpi2DLE, mpi2DLE, mpi2DLE, mpi2STX)
	want = append(want, ackFrame...)
	if !bytes.Equal(wire.writes.Bytes(), want) {
		t.Fatalf("writes=%x want=%x", wire.writes.Bytes(), want)
	}
}

func TestMPI2S7PDUExchangeRejectsInvalidBeforeIO(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		local byte
		pdu   []byte
	}{
		{name: "zero local", local: 0, pdu: mpi2TestSetupRequest()},
		{name: "invalid PDU", local: 3, pdu: []byte{0x32}},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wire := &mpi2InitWire{reader: bytes.NewReader(nil)}
			response, err := ExchangeMPI2S7PDU(wire, 0x55, tt.local, 4, tt.pdu)
			if err == nil || response != nil || wire.writes.Len() != 0 {
				t.Fatalf("response=%x err=%v writes=%x", response, err, wire.writes.Bytes())
			}
		})
	}
	if _, err := ExchangeMPI2S7PDU(nil, 0x55, 3, 4, mpi2TestSetupRequest()); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("nil wire err=%v", err)
	}
}

func TestMPI2S7PDUExchangeRejectsBadReply(t *testing.T) {
	t.Parallel()
	valid := mpi2TestPDUReplies(t)
	wrongAckPayload, _ := EncodeMPI2MessageAck(0x55, 3, 5)
	wrongAckFrame, _ := EncodeMPI2Frame(wrongAckPayload)
	validAckPayload, _ := EncodeMPI2MessageAck(0x55, 3, 4)
	validAckFrame, _ := EncodeMPI2Frame(validAckPayload)
	wrongAck := append([]byte{mpi2DLE, mpi2DLE, mpi2STX}, wrongAckFrame...)
	wrongAck = append(wrongAck, valid[3+len(validAckFrame):]...)
	badBCC := bytes.Clone(valid)
	ackTrailer := bytes.Index(badBCC, []byte{mpi2DLE, mpi2ETX})
	if ackTrailer < 0 {
		t.Fatal("missing acknowledgement trailer")
	}
	badBCC[ackTrailer+2] ^= 1
	responseStart := 3 + len(validAckFrame) + 1
	badResponsePayload, err := EncodeMPI2PDUEnvelope(0x55, 3, 9, mpi2TestSetupResponse())
	if err != nil {
		t.Fatal(err)
	}
	badResponsePayload[4] = 0 // valid frame, invalid envelope marker
	badResponseFrame, err := EncodeMPI2Frame(badResponsePayload)
	if err != nil {
		t.Fatal(err)
	}
	badResponse := append(bytes.Clone(valid[:responseStart]), badResponseFrame...)
	badResponse = append(badResponse, mpi2DLE, mpi2DLE)
	for _, tt := range []struct {
		name  string
		input []byte
		limit int
	}{
		{name: "start", input: []byte{mpi2STX}},
		{name: "request acknowledgement", input: []byte{mpi2DLE, mpi2STX}},
		{name: "ack start", input: []byte{mpi2DLE, mpi2DLE, mpi2DLE}},
		{name: "wrong acknowledgement number", input: wrongAck},
		{name: "bad acknowledgement BCC", input: badBCC},
		{name: "bad response", input: badResponse},
		{name: "truncated response", input: valid[:len(valid)-3]},
		{name: "missing final control", input: valid[:len(valid)-1]},
		{name: "short request write", input: []byte{mpi2DLE}, limit: 1},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wire := &mpi2InitWire{reader: bytes.NewReader(tt.input), limit: tt.limit}
			response, err := ExchangeMPI2S7PDU(wire, 0x55, 3, 4, mpi2TestSetupRequest())
			if err == nil || response != nil {
				t.Fatalf("accepted bad dialogue: response=%x err=%v", response, err)
			}
			if tt.name == "bad response" {
				request, _ := EncodeMPI2PDUEnvelope(0x55, 3, 4, mpi2TestSetupRequest())
				requestFrame, _ := EncodeMPI2Frame(request)
				if got, want := wire.writes.Len(), 1+len(requestFrame)+3; got != want {
					t.Fatalf("bad envelope was acknowledged: wrote %d bytes, want %d", got, want)
				}
			}
		})
	}
}
