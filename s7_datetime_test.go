package gos7

import (
	"bytes"
	"testing"
	"time"
)

func TestS7DateTimeStrictCodec(t *testing.T) {
	t.Parallel()
	want := time.Date(2026, time.September, 21, 12, 34, 56, 789_000_000, time.UTC)
	wire, err := EncodeS7DateTime(want)
	if err != nil {
		t.Fatal(err)
	}
	wantWire := []byte{0x26, 0x09, 0x21, 0x12, 0x34, 0x56, 0x78, 0x91}
	if !bytes.Equal(wire, wantWire) {
		t.Fatalf("wire=%x want=%x", wire, wantWire)
	}
	got, err := DecodeS7DateTime(wire)
	if err != nil || !got.Equal(want) {
		t.Fatalf("got=%s error=%v", got, err)
	}
}

func TestS7DateTimeStrictCodecRejectsMalformedValues(t *testing.T) {
	t.Parallel()
	for _, value := range []time.Time{
		time.Date(1989, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2090, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 1, 0, 0, 0, 1, time.UTC),
	} {
		if _, err := EncodeS7DateTime(value); err == nil {
			t.Fatalf("accepted %s", value)
		}
	}
	for _, wire := range [][]byte{
		nil,
		{0x26, 0x1a, 0x21, 0x12, 0x34, 0x56, 0x78, 0x91},
		{0x26, 0x02, 0x31, 0x12, 0x34, 0x56, 0x78, 0x92},
		{0x26, 0x09, 0x21, 0x12, 0x34, 0x56, 0x78, 0x92},
	} {
		if _, err := DecodeS7DateTime(wire); err == nil {
			t.Fatalf("accepted %x", wire)
		}
	}
}
