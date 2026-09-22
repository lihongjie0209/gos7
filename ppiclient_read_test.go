package gos7

import (
	"bytes"
	"testing"
)

func TestPPIReadPDUHighAddress(t *testing.T) {
	t.Parallel()
	request := PPIReadRequest{Area: 0x84, DBNumber: 5, Start: 1 << 18, Count: 1, PDUSize: 240, Reference: 7}
	pdu, err := EncodePPIReadPDU(request)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pdu[21:24], []byte{0x20, 0, 0}) {
		t.Fatalf("bit address=%x want 200000", pdu[21:24])
	}
	response := []byte{0x32, 3, 0, 0, 0, 7, 0, 2, 0, 5, 0, 0, 4, 1, 0xff, 4, 0, 8, 0x42}
	got, err := DecodePPIReadResponse(response, request)
	if err != nil || !bytes.Equal(got, []byte{0x42}) {
		t.Fatalf("got=%x error=%v", got, err)
	}
	response[len(response)-1] = 0
	if got[0] != 0x42 {
		t.Fatal("response data aliases input")
	}
}

func TestPPIReadPDUAddressBoundary(t *testing.T) {
	t.Parallel()
	valid := PPIReadRequest{Area: 0x83, Start: 1<<21 - 1, Count: 1, PDUSize: 240, Reference: 1}
	if _, err := EncodePPIReadPDU(valid); err != nil {
		t.Fatalf("last byte: %v", err)
	}
	for _, tt := range []struct {
		name string
		edit func(*PPIReadRequest)
	}{
		{"overflow", func(r *PPIReadRequest) { r.Start++ }},
		{"range overflow", func(r *PPIReadRequest) { r.Count++ }},
		{"bad area", func(r *PPIReadRequest) { r.Area = 0 }},
		{"DB on marker", func(r *PPIReadRequest) { r.DBNumber = 1 }},
		{"zero reference", func(r *PPIReadRequest) { r.Reference = 0 }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			request := valid
			tt.edit(&request)
			if _, err := EncodePPIReadPDU(request); err == nil {
				t.Fatalf("accepted %+v", request)
			}
		})
	}
}
