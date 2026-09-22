package gos7

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

type mpi2InitWire struct {
	reader *bytes.Reader
	writes bytes.Buffer
	limit  int
}

func (w *mpi2InitWire) Read(data []byte) (int, error) { return w.reader.Read(data) }

func (w *mpi2InitWire) Write(data []byte) (int, error) {
	if w.limit > 0 && len(data) > w.limit {
		data = data[:w.limit]
	}
	return w.writes.Write(data)
}

func TestMPI2AdapterInitPayload(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name         string
		baud         int
		code, timing byte
	}{
		{name: "9600", baud: 9600, code: 0, timing: 0x3c},
		{name: "19200", baud: 19200, code: 1, timing: 0x3c},
		{name: "187500", baud: 187500, code: 2, timing: 0x3c},
		{name: "500000", baud: 500000, code: 3, timing: 0x64},
		{name: "1500000", baud: 1500000, code: 4, timing: 0x96},
		{name: "45450", baud: 45450, code: 5, timing: 0x3c},
		{name: "93750", baud: 93750, code: 6, timing: 0x3c},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			payload, err := MPI2AdapterInitPayload(126, tt.baud)
			if err != nil || len(payload) != 23 || payload[7] != tt.timing || payload[15] != tt.code || payload[16] != 126 {
				t.Fatalf("payload=%x err=%v", payload, err)
			}
		})
	}
	if _, err := MPI2AdapterInitPayload(127, 187500); err == nil {
		t.Fatal("accepted invalid station")
	}
	if _, err := MPI2AdapterInitPayload(1, 38400); err == nil {
		t.Fatal("accepted unsupported speed")
	}
}

func TestMPI2AdapterInitialize(t *testing.T) {
	t.Parallel()
	frame, err := EncodeMPI2Frame([]byte{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	input := append([]byte{0x10, 0x10, 0x02}, frame...)
	wire := &mpi2InitWire{reader: bytes.NewReader(input)}
	response, err := InitializeMPI2Adapter(wire, 4, 187500)
	if err != nil || !bytes.Equal(response, []byte{1, 2, 3}) {
		t.Fatalf("response=%x err=%v", response, err)
	}
	config, err := MPI2AdapterInitPayload(4, 187500)
	if err != nil {
		t.Fatal(err)
	}
	configFrame, err := EncodeMPI2Frame(config)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]byte{0x02}, configFrame...)
	want = append(want, 0x10, 0x10)
	if !bytes.Equal(wire.writes.Bytes(), want) || wire.reader.Len() != 0 {
		t.Fatalf("writes=%x want=%x unread=%d", wire.writes.Bytes(), want, wire.reader.Len())
	}
}

func TestMPI2AdapterInitializeRejectsBadDialogue(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		input   []byte
		station byte
		baud    int
		limit   int
	}{
		{name: "invalid station", station: 127, baud: 187500},
		{name: "invalid speed", station: 1, baud: 38400},
		{name: "bad start", input: []byte{0x02}, baud: 187500},
		{name: "bad config ack", input: []byte{0x10, 0x02}, baud: 187500},
		{name: "bad response start", input: []byte{0x10, 0x10, 0x10}, baud: 187500},
		{name: "truncated frame", input: []byte{0x10, 0x10, 0x02, 1, 0x10, 3}, baud: 187500},
		{name: "bad bcc", input: []byte{0x10, 0x10, 0x02, 1, 0x10, 3, 0}, baud: 187500},
		{name: "short write", input: []byte{0x10}, baud: 187500, limit: 1},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wire := &mpi2InitWire{reader: bytes.NewReader(tt.input), limit: tt.limit}
			response, err := InitializeMPI2Adapter(wire, tt.station, tt.baud)
			if err == nil || response != nil {
				t.Fatalf("accepted bad dialogue: response=%x err=%v", response, err)
			}
			if tt.name == "invalid station" || tt.name == "invalid speed" {
				if wire.writes.Len() != 0 {
					t.Fatalf("wrote before validation: %x", wire.writes.Bytes())
				}
			}
		})
	}
	if _, err := InitializeMPI2Adapter(nil, 0, 187500); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("nil transport error=%v", err)
	}
}
