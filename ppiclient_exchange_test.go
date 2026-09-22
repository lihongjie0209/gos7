package gos7

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

type ppiTestWire struct {
	reads  *bytes.Reader
	writes bytes.Buffer
	limit  int
	short  bool
}

func (w *ppiTestWire) Read(p []byte) (int, error) {
	if w.limit > 0 && len(p) > w.limit {
		p = p[:w.limit]
	}
	return w.reads.Read(p)
}

func (w *ppiTestWire) Write(p []byte) (int, error) {
	if w.short {
		return 0, nil
	}
	return w.writes.Write(p)
}

func TestPPIExchange(t *testing.T) {
	t.Parallel()
	response, err := EncodePPIFrame(PPIFrame{Kind: PPIVariable, Destination: 0, Source: 2, Control: 0x08, Payload: []byte{0x32, 0x03}})
	if err != nil {
		t.Fatal(err)
	}
	wire := &ppiTestWire{reads: bytes.NewReader(append([]byte{0xe5, 0xe5}, response...)), limit: 1}
	got, err := ExchangePPI(wire, 0, 2, []byte{0x32, 0x01})
	if err != nil || !bytes.Equal(got, []byte{0x32, 0x03}) {
		t.Fatalf("exchange=%x error=%v", got, err)
	}
	request, _ := EncodePPIFrame(PPIFrame{Kind: PPIVariable, Destination: 2, Source: 0, Control: 0x6c, Payload: []byte{0x32, 0x01}})
	poll, _ := EncodePPIFrame(PPIFrame{Kind: PPIFixed, Destination: 2, Source: 0, Control: 0x5c})
	alternate, _ := EncodePPIFrame(PPIFrame{Kind: PPIFixed, Destination: 2, Source: 0, Control: 0x7c})
	want := append(append(append([]byte{}, request...), poll...), alternate...)
	if !bytes.Equal(wire.writes.Bytes(), want) {
		t.Fatalf("writes=%x want=%x", wire.writes.Bytes(), want)
	}
}

func TestPPIExchangeRejectsInvalidResponses(t *testing.T) {
	t.Parallel()
	wrongAddress, _ := EncodePPIFrame(PPIFrame{Kind: PPIVariable, Destination: 1, Source: 2, Payload: []byte{0x32}})
	emptyPayload, _ := EncodePPIFrame(PPIFrame{Kind: PPIVariable, Destination: 0, Source: 2})
	for _, tt := range []struct {
		name string
		data []byte
	}{
		{"no acknowledgement", []byte{0x10, 2, 0, 0x5c, 0x5e, 0x16}},
		{"wrong address", append([]byte{0xe5}, wrongAddress...)},
		{"empty payload", append([]byte{0xe5}, emptyPayload...)},
		{"too many polls", bytes.Repeat([]byte{0xe5}, 6)},
		{"truncated", []byte{0xe5, 0x68, 4}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wire := &ppiTestWire{reads: bytes.NewReader(tt.data), limit: 1}
			if _, err := ExchangePPI(wire, 0, 2, []byte{0x32}); err == nil {
				t.Fatalf("accepted invalid exchange %x", tt.data)
			}
		})
	}
}

func TestPPIExchangeRejectsInvalidRequestAndShortWrite(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name          string
		local, target byte
		payload       []byte
		short         bool
	}{
		{"empty", 0, 2, nil, false},
		{"same stations", 2, 2, []byte{0x32}, false},
		{"invalid station", 127, 2, []byte{0x32}, false},
		{"short write", 0, 2, []byte{0x32}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wire := &ppiTestWire{reads: bytes.NewReader([]byte{0xe5}), short: tt.short}
			if _, err := ExchangePPI(wire, tt.local, tt.target, tt.payload); err == nil {
				t.Fatal("accepted invalid request or short write")
			}
		})
	}
}

func TestReadPPIFrame(t *testing.T) {
	t.Parallel()
	for _, data := range [][]byte{{0xe5}, {0x10, 2, 0, 0x5c, 0x5e, 0x16}} {
		got, err := ReadPPIFrame(&ppiTestWire{reads: bytes.NewReader(data), limit: 1})
		if err != nil || !bytes.Equal(got, data) {
			t.Fatalf("read=%x error=%v want=%x", got, err, data)
		}
	}
	if _, err := ReadPPIFrame(strings.NewReader("\x00")); err == nil {
		t.Fatal("accepted unknown start")
	}
	if _, err := ReadPPIFrame(strings.NewReader("\x68\xff")); err != io.ErrUnexpectedEOF {
		t.Fatalf("truncated=%v want unexpected EOF", err)
	}
}
