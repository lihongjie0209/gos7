package gos7

import (
	"bytes"
	"testing"
)

func TestPPIWritePDUHighAddress(t *testing.T) {
	t.Parallel()
	request := PPIWriteRequest{Area: 0x84, DBNumber: 5, Start: 1 << 18, Data: []byte{0x11, 0x22}, PDUSize: 240, Reference: 7}
	pdu, err := EncodePPIWritePDU(request)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pdu[21:24], []byte{0x20, 0, 0}) || !bytes.Equal(pdu[28:], request.Data) {
		t.Fatalf("address=%x payload=%x", pdu[21:24], pdu[28:])
	}
	response := []byte{0x32, 3, 0, 0, 0, 7, 0, 2, 0, 1, 0, 0, 5, 1, 0xff}
	if err := DecodePPIWriteResponse(response, request); err != nil {
		t.Fatal(err)
	}
	response[14] = 5
	if err := DecodePPIWriteResponse(response, request); err == nil {
		t.Fatal("accepted failed write acknowledgement")
	}
}

func TestPPIWritePDUAddressBoundary(t *testing.T) {
	t.Parallel()
	valid := PPIWriteRequest{Area: 0x83, Start: 1<<21 - 1, Data: []byte{1}, PDUSize: 240, Reference: 1}
	if _, err := EncodePPIWritePDU(valid); err != nil {
		t.Fatalf("last byte: %v", err)
	}
	for _, tt := range []struct {
		name string
		edit func(*PPIWriteRequest)
	}{
		{"overflow", func(r *PPIWriteRequest) { r.Start++ }},
		{"range overflow", func(r *PPIWriteRequest) { r.Data = []byte{1, 2} }},
		{"bad area", func(r *PPIWriteRequest) { r.Area = 0 }},
		{"DB on marker", func(r *PPIWriteRequest) { r.DBNumber = 1 }},
		{"zero reference", func(r *PPIWriteRequest) { r.Reference = 0 }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			request := valid
			tt.edit(&request)
			if _, err := EncodePPIWritePDU(request); err == nil {
				t.Fatalf("accepted %+v", request)
			}
		})
	}
}
