package gos7

import (
	"bytes"
	"testing"
)

func TestS7StringStrictCodec(t *testing.T) {
	t.Parallel()
	wire, err := EncodeS7String(8, "温度")
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{8, 6, 0xe6, 0xb8, 0xa9, 0xe5, 0xba, 0xa6, 0, 0}
	if !bytes.Equal(wire, want) {
		t.Fatalf("wire=%x want=%x", wire, want)
	}
	value, err := DecodeS7String(wire, 8)
	if err != nil || value != "温度" {
		t.Fatalf("value=%q error=%v", value, err)
	}
	wire[2] = 'x'
	if value != "温度" {
		t.Fatal("decoded value aliases wire")
	}
}

func TestS7StringStrictCodecRejectsMalformedValues(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		maximum int
		value   string
	}{
		{0, "x"}, {255, "x"}, {1, "xx"}, {2, string([]byte{0xff})},
	} {
		if _, err := EncodeS7String(test.maximum, test.value); err == nil {
			t.Fatalf("accepted max=%d value=%q", test.maximum, test.value)
		}
	}
	for _, test := range []struct {
		wire    []byte
		maximum int
	}{
		{nil, 1},
		{[]byte{2, 1, 'x'}, 1},
		{[]byte{1, 2, 'x'}, 1},
		{[]byte{1, 1, 0xff}, 1},
		{[]byte{2, 1, 'x', 1}, 2},
	} {
		if _, err := DecodeS7String(test.wire, test.maximum); err == nil {
			t.Fatalf("accepted wire=%x max=%d", test.wire, test.maximum)
		}
	}
}
