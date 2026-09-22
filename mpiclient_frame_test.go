package gos7

import (
	"bytes"
	"io"
	"testing"
)

func TestMPI2FrameRoundTrip(t *testing.T) {
	t.Parallel()
	payload := []byte{1, 0x10, 3}
	want := []byte{1, 0x10, 0x10, 3, 0x10, 3, 0x11}
	wire, err := EncodeMPI2Frame(payload)
	if err != nil || !bytes.Equal(wire, want) {
		t.Fatalf("wire=%x error=%v want=%x", wire, err, want)
	}
	payload[0] = 0xff
	decoded, err := DecodeMPI2Frame(wire)
	if err != nil || !bytes.Equal(decoded, []byte{1, 0x10, 3}) {
		t.Fatalf("decoded=%x error=%v", decoded, err)
	}
	wire[0] = 0xff
	if decoded[0] != 1 {
		t.Fatal("decoded payload aliases wire")
	}
}

func TestMPI2FrameRejectsMalformed(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		wire []byte
	}{
		{name: "empty"},
		{name: "truncated escape", wire: []byte{1, 0x10}},
		{name: "invalid escape", wire: []byte{1, 0x10, 2, 0x10, 3, 0}},
		{name: "bad BCC", wire: []byte{1, 0x10, 3, 0}},
		{name: "trailing bytes", wire: []byte{1, 0x10, 3, 0x12, 0}},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if payload, err := DecodeMPI2Frame(tt.wire); err == nil || payload != nil {
				t.Fatalf("accepted %x: payload=%x error=%v", tt.wire, payload, err)
			}
		})
	}
	if _, err := EncodeMPI2Frame(nil); err == nil {
		t.Fatal("accepted empty payload")
	}
	if _, err := EncodeMPI2Frame(bytes.Repeat([]byte{1}, 2049)); err == nil {
		t.Fatal("accepted oversized payload")
	}
}

func TestMPI2FrameStreaming(t *testing.T) {
	t.Parallel()
	first, _ := EncodeMPI2Frame([]byte{0x10, 0x42})
	second, _ := EncodeMPI2Frame([]byte{0x21})
	reader := oneByteMPI2Reader{bytes.NewReader(append(first, second...))}
	for _, want := range [][]byte{{0x10, 0x42}, {0x21}} {
		got, err := ReadMPI2Frame(reader)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("got=%x error=%v want=%x", got, err, want)
		}
	}
}

type oneByteMPI2Reader struct{ reader io.Reader }

func (r oneByteMPI2Reader) Read(data []byte) (int, error) {
	return r.reader.Read(data[:min(len(data), 1)])
}
