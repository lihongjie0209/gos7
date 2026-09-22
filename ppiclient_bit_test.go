package gos7

import (
	"bytes"
	"testing"
)

func ppiTestBitRequest() PPIBitRequest {
	return PPIBitRequest{Area: 0x84, DBNumber: 5, Start: 2, Bit: 3, PDUSize: 192, Reference: 7}
}

func ppiTestBitReadResponse(value byte) []byte {
	return []byte{0x32, 3, 0, 0, 0, 7, 0, 2, 0, 5, 0, 0, 4, 1, 0xff, 3, 0, 1, value}
}

func TestPPIBitReadCodec(t *testing.T) {
	t.Parallel()
	request := ppiTestBitRequest()
	pdu, err := EncodePPIReadBitPDU(request)
	want := []byte{0x32, 1, 0, 0, 0, 7, 0, 14, 0, 0, 4, 1, 0x12, 0x0a, 0x10, 1, 0, 1, 0, 5, 0x84, 0, 0, 19}
	if err != nil || !bytes.Equal(pdu, want) {
		t.Fatalf("PDU=%x err=%v want=%x", pdu, err, want)
	}
	for _, tt := range []struct {
		name  string
		value byte
		want  bool
	}{
		{name: "false", value: 0, want: false},
		{name: "true", value: 1, want: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodePPIReadBitResponse(ppiTestBitReadResponse(tt.value), request)
			if err != nil || got != tt.want {
				t.Fatalf("value=%v err=%v want=%v", got, err, tt.want)
			}
		})
	}
}

func TestPPIBitWriteCodec(t *testing.T) {
	t.Parallel()
	request := ppiTestBitRequest()
	pdu, err := EncodePPIWriteBitPDU(request, true)
	want := []byte{0x32, 1, 0, 0, 0, 7, 0, 14, 0, 5, 5, 1, 0x12, 0x0a, 0x10, 1, 0, 1, 0, 5, 0x84, 0, 0, 19, 0, 3, 0, 1, 1}
	if err != nil || !bytes.Equal(pdu, want) {
		t.Fatalf("PDU=%x err=%v want=%x", pdu, err, want)
	}
	if err := DecodePPIWriteBitResponse(mpi2TestWriteResponse(), request); err != nil {
		t.Fatal(err)
	}
}

func TestPPIBitRejectsInvalidRequestsAndResponses(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		edit func(*PPIBitRequest)
	}{
		{name: "area", edit: func(r *PPIBitRequest) { r.Area = 0 }},
		{name: "DB on marker", edit: func(r *PPIBitRequest) { r.Area, r.DBNumber = 0x83, 1 }},
		{name: "negative DB", edit: func(r *PPIBitRequest) { r.DBNumber = -1 }},
		{name: "DB overflow", edit: func(r *PPIBitRequest) { r.DBNumber = 65536 }},
		{name: "negative start", edit: func(r *PPIBitRequest) { r.Start = -1 }},
		{name: "start overflow", edit: func(r *PPIBitRequest) { r.Start = 1 << 21 }},
		{name: "bit overflow", edit: func(r *PPIBitRequest) { r.Bit = 8 }},
		{name: "small PDU", edit: func(r *PPIBitRequest) { r.PDUSize = 63 }},
		{name: "large PDU", edit: func(r *PPIBitRequest) { r.PDUSize = 241 }},
		{name: "zero reference", edit: func(r *PPIBitRequest) { r.Reference = 0 }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			request := ppiTestBitRequest()
			tt.edit(&request)
			if pdu, err := EncodePPIReadBitPDU(request); err == nil || pdu != nil {
				t.Fatalf("read accepted %+v: %x %v", request, pdu, err)
			}
			if pdu, err := EncodePPIWriteBitPDU(request, true); err == nil || pdu != nil {
				t.Fatalf("write accepted %+v: %x %v", request, pdu, err)
			}
		})
	}
	for _, tt := range []struct {
		name string
		edit func([]byte) []byte
	}{
		{name: "short", edit: func(p []byte) []byte { return p[:18] }},
		{name: "header", edit: func(p []byte) []byte { p[0] = 0; return p }},
		{name: "reference", edit: func(p []byte) []byte { p[5] = 2; return p }},
		{name: "data length", edit: func(p []byte) []byte { p[9] = 4; return p }},
		{name: "function", edit: func(p []byte) []byte { p[12] = 5; return p }},
		{name: "status", edit: func(p []byte) []byte { p[14] = 5; return p }},
		{name: "transport", edit: func(p []byte) []byte { p[15] = 4; return p }},
		{name: "length", edit: func(p []byte) []byte { p[17] = 8; return p }},
		{name: "value", edit: func(p []byte) []byte { p[18] = 2; return p }},
		{name: "trailing", edit: func(p []byte) []byte { return append(p, 0) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if value, err := DecodePPIReadBitResponse(tt.edit(ppiTestBitReadResponse(1)), ppiTestBitRequest()); err == nil || value {
				t.Fatalf("accepted malformed response: value=%v err=%v", value, err)
			}
		})
	}
}

func TestPPIBitOperationsRejectBeforeIO(t *testing.T) {
	t.Parallel()
	request := ppiTestBitRequest()
	request.Bit = 8
	wire := &mpi2InitWire{reader: bytes.NewReader(nil)}
	if value, err := ReadPPIBit(wire, 0x55, 3, 4, request); err == nil || value || wire.writes.Len() != 0 {
		t.Fatalf("invalid read caused I/O: value=%v err=%v writes=%x", value, err, wire.writes.Bytes())
	}
	if err := WritePPIBit(wire, 0x55, 3, 4, request, true); err == nil || wire.writes.Len() != 0 {
		t.Fatalf("invalid write caused I/O: err=%v writes=%x", err, wire.writes.Bytes())
	}
}
