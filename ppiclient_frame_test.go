package gos7

import (
	"bytes"
	"testing"
)

func TestPPIFrameRoundTrip(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		frame PPIFrame
		wire  []byte
	}{
		{"ack", PPIFrame{Kind: PPIAck}, []byte{0xe5}},
		{"fixed", PPIFrame{Kind: PPIFixed, Destination: 2, Source: 0, Control: 0x5c}, []byte{0x10, 2, 0, 0x5c, 0x5e, 0x16}},
		{"variable", PPIFrame{Kind: PPIVariable, Destination: 2, Source: 0, Control: 0x6c, Payload: []byte{0x32, 0x01}}, []byte{0x68, 5, 5, 0x68, 2, 0, 0x6c, 0x32, 0x01, 0xa1, 0x16}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wire, err := EncodePPIFrame(tt.frame)
			if err != nil || !bytes.Equal(wire, tt.wire) {
				t.Fatalf("wire=%x error=%v want=%x", wire, err, tt.wire)
			}
			decoded, err := DecodePPIFrame(wire)
			if err != nil || decoded.Kind != tt.frame.Kind || decoded.Destination != tt.frame.Destination || decoded.Source != tt.frame.Source || decoded.Control != tt.frame.Control || !bytes.Equal(decoded.Payload, tt.frame.Payload) {
				t.Fatalf("decoded=%+v error=%v want=%+v", decoded, err, tt.frame)
			}
		})
	}
}

func TestPPIFrameBoundsAndOwnership(t *testing.T) {
	t.Parallel()
	for _, frame := range []PPIFrame{
		{Kind: PPIFixed, Destination: 127},
		{Kind: PPIVariable, Source: 255},
		{Kind: PPIVariable, Payload: make([]byte, 253)},
		{Kind: PPIAck, Control: 1},
	} {
		if _, err := EncodePPIFrame(frame); err == nil {
			t.Fatalf("accepted invalid frame %+v", frame)
		}
	}
	wire, err := EncodePPIFrame(PPIFrame{Kind: PPIVariable, Destination: 126, Payload: make([]byte, 252)})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodePPIFrame(wire)
	if err != nil || len(decoded.Payload) != 252 {
		t.Fatalf("decoded=%+v error=%v", decoded, err)
	}
	wire[7] = 1
	if decoded.Payload[0] != 0 {
		t.Fatal("decoded payload aliases input")
	}
}

func TestPPIFrameRejectsMalformed(t *testing.T) {
	t.Parallel()
	valid := []byte{0x68, 5, 5, 0x68, 2, 0, 0x6c, 0x32, 0x01, 0xa1, 0x16}
	for _, tt := range []struct {
		name string
		wire []byte
	}{
		{"empty", nil},
		{"unknown start", []byte{0}},
		{"ack trailing", []byte{0xe5, 0}},
		{"short fixed", []byte{0x10, 2}},
		{"fixed checksum", []byte{0x10, 2, 0, 0x5c, 0, 0x16}},
		{"fixed terminator", []byte{0x10, 2, 0, 0x5c, 0x5e, 0}},
		{"length mismatch", append([]byte{0x68, 5, 4}, valid[3:]...)},
		{"second start", append([]byte{0x68, 5, 5, 0}, valid[4:]...)},
		{"short length", []byte{0x68, 2, 2, 0x68, 2, 0, 2, 0x16}},
		{"truncated", valid[:len(valid)-1]},
		{"trailing", append(append([]byte{}, valid...), 0)},
		{"checksum", append(append([]byte{}, valid[:9]...), 0, 0x16)},
		{"terminator", append(append([]byte{}, valid[:10]...), 0)},
		{"destination", []byte{0x10, 127, 0, 0x5c, 0xdb, 0x16}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := DecodePPIFrame(tt.wire); err == nil {
				t.Fatalf("accepted malformed frame %x", tt.wire)
			}
		})
	}
}
