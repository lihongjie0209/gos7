package gos7

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func mpi2AcceptFrame(t *testing.T, station byte) []byte {
	t.Helper()
	frame, err := EncodeMPI2Frame([]byte{
		0, 0x0c, 0x44, 0x55, 0xd0, 4, 0, 0x80,
		1, 6, 0, 2, 0, 1, 2, station, 1, 0, 1, 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	return frame
}

func mpi2ConfirmReplyFrame(t *testing.T) []byte {
	t.Helper()
	frame, err := EncodeMPI2Frame([]byte{0, 0x0c, 0xaa, 0xbb, 5, 1})
	if err != nil {
		t.Fatal(err)
	}
	return frame
}

func TestMPI2PLCConnectionExchange(t *testing.T) {
	t.Parallel()
	input := append([]byte{mpi2DLE, mpi2DLE, mpi2STX}, mpi2AcceptFrame(t, 2)...)
	input = append(input, mpi2DLE, mpi2DLE, mpi2STX)
	input = append(input, mpi2ConfirmReplyFrame(t)...)
	wire := &mpi2InitWire{reader: bytes.NewReader(input)}
	peer, err := ExchangeMPI2PLCConnection(wire, 2, 3)
	if err != nil || peer != 0x55 || wire.reader.Len() != 0 {
		t.Fatalf("peer=%x err=%v unread=%d", peer, err, wire.reader.Len())
	}
	offer, err := EncodeMPI2PLCConnectOffer(2, 3)
	if err != nil {
		t.Fatal(err)
	}
	offerFrame, err := EncodeMPI2Frame(offer)
	if err != nil {
		t.Fatal(err)
	}
	confirm, err := EncodeMPI2PLCConnectConfirm(0x55, 3)
	if err != nil {
		t.Fatal(err)
	}
	confirmFrame, err := EncodeMPI2Frame(confirm)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]byte{mpi2STX}, offerFrame...)
	want = append(want, mpi2DLE, mpi2DLE, mpi2STX)
	want = append(want, confirmFrame...)
	want = append(want, mpi2DLE, mpi2DLE)
	if !bytes.Equal(wire.writes.Bytes(), want) {
		t.Fatalf("writes=%x want=%x", wire.writes.Bytes(), want)
	}
}

func TestMPI2PLCConnectionPhases(t *testing.T) {
	t.Parallel()
	input := append([]byte{mpi2DLE, mpi2DLE, mpi2STX}, mpi2AcceptFrame(t, 2)...)
	input = append(input, mpi2DLE, mpi2DLE, mpi2STX)
	input = append(input, mpi2ConfirmReplyFrame(t)...)
	wire := &mpi2InitWire{reader: bytes.NewReader(input)}
	peer, err := ExchangeMPI2PLCConnectOffer(wire, 2, 3)
	if err != nil || peer != 0x55 {
		t.Fatalf("offer peer=%x err=%v", peer, err)
	}
	if err := ExchangeMPI2PLCConnectConfirm(wire, peer, 3); err != nil || wire.reader.Len() != 0 {
		t.Fatalf("confirm err=%v unread=%d", err, wire.reader.Len())
	}
}

func TestMPI2PLCConnectionRejectsInvalidBeforeIO(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name           string
		station, local byte
	}{
		{name: "station", station: 127, local: 3},
		{name: "connection", station: 2, local: 0},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wire := &mpi2InitWire{reader: bytes.NewReader(nil)}
			peer, err := ExchangeMPI2PLCConnection(wire, tt.station, tt.local)
			if err == nil || peer != 0 || wire.writes.Len() != 0 {
				t.Fatalf("peer=%x err=%v writes=%x", peer, err, wire.writes.Bytes())
			}
		})
	}
	if _, err := ExchangeMPI2PLCConnection(nil, 2, 3); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("nil wire err=%v", err)
	}
}

func TestMPI2PLCConnectionRejectsBadDialogue(t *testing.T) {
	t.Parallel()
	valid := mpi2AcceptFrame(t, 2)
	badBCC := bytes.Clone(valid)
	badBCC[len(badBCC)-1] ^= 1
	badConfirm, err := EncodeMPI2Frame([]byte{0, 0x0c, 0xaa, 0xbb, 5, 2})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name  string
		input []byte
		limit int
	}{
		{name: "start control", input: []byte{mpi2STX}},
		{name: "offer ack", input: []byte{mpi2DLE, mpi2STX}},
		{name: "offer start", input: []byte{mpi2DLE, mpi2DLE, mpi2DLE}},
		{name: "offer frame", input: append([]byte{mpi2DLE, mpi2DLE, mpi2STX}, badBCC...)},
		{name: "wrong station", input: append([]byte{mpi2DLE, mpi2DLE, mpi2STX}, mpi2AcceptFrame(t, 3)...)},
		{name: "short offer write", input: []byte{mpi2DLE}, limit: 1},
		{name: "confirm pending ack", input: append(append([]byte{mpi2DLE, mpi2DLE, mpi2STX}, valid...), mpi2STX)},
		{name: "bad confirmation", input: append(append([]byte{mpi2DLE, mpi2DLE, mpi2STX}, valid...), append([]byte{mpi2DLE, mpi2DLE, mpi2STX}, badConfirm...)...)},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wire := &mpi2InitWire{reader: bytes.NewReader(tt.input), limit: tt.limit}
			peer, err := ExchangeMPI2PLCConnection(wire, 2, 3)
			if err == nil || peer != 0 {
				t.Fatalf("accepted bad dialogue: peer=%x err=%v", peer, err)
			}
		})
	}
}
