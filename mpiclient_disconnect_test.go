package gos7

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

type mpi2DisconnectTraceWire struct {
	*mpi2InitWire
	thirdReadWrites []byte
	reads           int
	t               *testing.T
}

func (w *mpi2DisconnectTraceWire) Read(data []byte) (int, error) {
	if w.reads == 2 && !bytes.Equal(w.writes.Bytes(), w.thirdReadWrites) {
		w.t.Fatalf("wrong control order before reply start: writes=%x want=%x", w.writes.Bytes(), w.thirdReadWrites)
	}
	w.reads++
	return w.mpi2InitWire.Read(data)
}

func TestMPI2DisconnectPayloads(t *testing.T) {
	t.Parallel()
	plc, err := MPI2PLCDisconnectPayload(0x55, 3)
	if err != nil || !bytes.Equal(plc, []byte{0, 0x0c, 0x55, 3, 0x80}) {
		t.Fatalf("PLC payload=%x err=%v", plc, err)
	}
	plc[0] = 1
	again, _ := MPI2PLCDisconnectPayload(0x55, 3)
	if again[0] != 0 {
		t.Fatal("PLC payload aliases shared storage")
	}
	adapter := MPI2AdapterShutdownPayload()
	if !bytes.Equal(adapter, []byte{1, 4, 2}) {
		t.Fatalf("adapter payload=%x", adapter)
	}
	adapter[0] = 0
	if MPI2AdapterShutdownPayload()[0] != 1 {
		t.Fatal("adapter payload aliases shared storage")
	}
	if payload, err := MPI2PLCDisconnectPayload(0x55, 0); err == nil || payload != nil {
		t.Fatalf("accepted zero local connection: %x %v", payload, err)
	}
}

func TestMPI2DisconnectRequestDialogues(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		request func(io.ReadWriter) ([]byte, error)
		payload []byte
	}{
		{name: "PLC", request: func(w io.ReadWriter) ([]byte, error) { return RequestMPI2PLCDisconnect(w, 0x55, 3) }, payload: []byte{0, 0x0c, 0x55, 3, 0x80}},
		{name: "adapter", request: RequestMPI2AdapterShutdown, payload: []byte{1, 4, 2}},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			responseFrame, err := EncodeMPI2Frame([]byte{0xa1, 0xb2, 0xc3})
			if err != nil {
				t.Fatal(err)
			}
			input := append([]byte{mpi2DLE, mpi2DLE, mpi2STX}, responseFrame...)
			requestFrame, err := EncodeMPI2Frame(tt.payload)
			if err != nil {
				t.Fatal(err)
			}
			want := append([]byte{mpi2STX}, requestFrame...)
			beforeReply := bytes.Clone(want)
			if tt.name == "PLC" {
				beforeReply = append(beforeReply, mpi2DLE)
			}
			wire := &mpi2DisconnectTraceWire{
				mpi2InitWire:    &mpi2InitWire{reader: bytes.NewReader(input)},
				thirdReadWrites: beforeReply,
				t:               t,
			}
			response, err := tt.request(wire)
			if err != nil || !bytes.Equal(response, []byte{0xa1, 0xb2, 0xc3}) || wire.reader.Len() != 0 {
				t.Fatalf("response=%x err=%v unread=%d", response, err, wire.reader.Len())
			}
			responseFrame[0] = 0
			if response[0] != 0xa1 {
				t.Fatal("response aliases wire storage")
			}
			want = append(want, mpi2DLE, mpi2DLE)
			if !bytes.Equal(wire.writes.Bytes(), want) {
				t.Fatalf("writes=%x want=%x", wire.writes.Bytes(), want)
			}
		})
	}
}

func TestMPI2DisconnectRequestsRejectBadDialogue(t *testing.T) {
	t.Parallel()
	frame, _ := EncodeMPI2Frame([]byte{1, 2, 3})
	badFrame := bytes.Clone(frame)
	badFrame[len(badFrame)-1] ^= 1
	for _, tt := range []struct {
		name  string
		input []byte
		limit int
	}{
		{name: "bad start", input: []byte{mpi2STX}},
		{name: "bad request ack", input: []byte{mpi2DLE, mpi2STX}},
		{name: "bad reply start", input: []byte{mpi2DLE, mpi2DLE, mpi2DLE}},
		{name: "bad frame checksum", input: append([]byte{mpi2DLE, mpi2DLE, mpi2STX}, badFrame...)},
		{name: "short write", input: []byte{mpi2DLE}, limit: 1},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, request := range []func(io.ReadWriter) ([]byte, error){
				func(w io.ReadWriter) ([]byte, error) { return RequestMPI2PLCDisconnect(w, 0x55, 3) },
				RequestMPI2AdapterShutdown,
			} {
				wire := &mpi2InitWire{reader: bytes.NewReader(tt.input), limit: tt.limit}
				if response, err := request(wire); err == nil || response != nil {
					t.Fatalf("accepted bad dialogue: response=%x err=%v", response, err)
				}
			}
		})
	}
	wire := &mpi2InitWire{reader: bytes.NewReader(nil)}
	if response, err := RequestMPI2PLCDisconnect(wire, 0x55, 0); err == nil || response != nil || wire.writes.Len() != 0 {
		t.Fatalf("invalid PLC request caused I/O: response=%x err=%v writes=%x", response, err, wire.writes.Bytes())
	}
	if _, err := RequestMPI2PLCDisconnect(nil, 0x55, 3); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("nil PLC wire error=%v", err)
	}
	if _, err := RequestMPI2AdapterShutdown(nil); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("nil adapter wire error=%v", err)
	}
}
