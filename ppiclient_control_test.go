package gos7

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func ppiCPUControlResponse(reference uint16, function byte, state ...byte) []byte {
	response := make([]byte, 13+len(state))
	response[0], response[1] = 0x32, 3
	binary.BigEndian.PutUint16(response[4:6], reference)
	binary.BigEndian.PutUint16(response[6:8], uint16(1+len(state)))
	response[12] = function
	copy(response[13:], state)
	return response
}

func ppiCPUStatusResponse(reference uint16, status byte) []byte {
	response := make([]byte, 38)
	response[0], response[1] = 0x32, 7
	binary.BigEndian.PutUint16(response[4:6], reference)
	binary.BigEndian.PutUint16(response[6:8], 8)
	binary.BigEndian.PutUint16(response[8:10], 20)
	response[37] = status
	return response
}

func TestPPICPUControlCodecUsesEstablishedTelegrams(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name      string
		operation PPICPUOperation
		template  []byte
		function  byte
	}{
		{name: "hot start", operation: PPICPUHotStart, template: s7HotStartTelegram, function: pduStart},
		{name: "cold start", operation: PPICPUColdStart, template: s7ColdStartTelegram, function: pduStart},
		{name: "stop", operation: PPICPUStop, template: s7StopTelegram, function: pduStop},
	} {
		t.Run(tt.name, func(t *testing.T) {
			request := PPICPUControlRequest{Operation: tt.operation, PDUSize: 240, Reference: 7}
			pdu, err := EncodePPICPUControlPDU(request)
			want := bytes.Clone(tt.template[7:])
			binary.BigEndian.PutUint16(want[4:6], 7)
			if err != nil || !bytes.Equal(pdu, want) {
				t.Fatalf("PDU=%x err=%v want=%x", pdu, err, want)
			}
			if err := DecodePPICPUControlResponse(ppiCPUControlResponse(7, tt.function), request); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPPICPUStatusCodecUsesEstablishedTelegram(t *testing.T) {
	t.Parallel()
	request := PPICPUStatusRequest{PDUSize: 240, Reference: 9}
	pdu, err := EncodePPICPUStatusPDU(request)
	want := bytes.Clone(s7GetStatusTelegram[7:])
	binary.BigEndian.PutUint16(want[4:6], 9)
	if err != nil || !bytes.Equal(pdu, want) {
		t.Fatalf("PDU=%x err=%v want=%x", pdu, err, want)
	}
	for _, tt := range []struct {
		wire byte
		want PPICPUStatus
	}{
		{wire: 8, want: PPICPUStatusRunning},
		{wire: 4, want: PPICPUStatusStopped},
		{wire: 3, want: PPICPUStatusStopped},
		{wire: 0, want: PPICPUStatusUnknown},
	} {
		got, err := DecodePPICPUStatusResponse(ppiCPUStatusResponse(9, tt.wire), request)
		if err != nil || got != tt.want {
			t.Fatalf("wire=%d status=%v err=%v want=%v", tt.wire, got, err, tt.want)
		}
	}
}

func TestPPICPUCodecRejectsInvalidRequestsAndResponses(t *testing.T) {
	t.Parallel()
	for _, request := range []PPICPUControlRequest{
		{Operation: 99, PDUSize: 240, Reference: 1},
		{Operation: PPICPUStop, PDUSize: 63, Reference: 1},
		{Operation: PPICPUStop, PDUSize: 241, Reference: 1},
		{Operation: PPICPUStop, PDUSize: 240},
	} {
		if pdu, err := EncodePPICPUControlPDU(request); err == nil || pdu != nil {
			t.Fatalf("accepted control request %+v: %x %v", request, pdu, err)
		}
	}
	if pdu, err := EncodePPICPUStatusPDU(PPICPUStatusRequest{PDUSize: 64}); err == nil || pdu != nil {
		t.Fatalf("accepted status request: %x %v", pdu, err)
	}
	validControl := PPICPUControlRequest{Operation: PPICPUStop, PDUSize: 240, Reference: 7}
	for _, mutate := range []func([]byte) []byte{
		func(p []byte) []byte { return p[:12] },
		func(p []byte) []byte { p[0] = 0; return p },
		func(p []byte) []byte { p[5] = 8; return p },
		func(p []byte) []byte { p[7] = 2; return p },
		func(p []byte) []byte { p[10] = 1; return p },
		func(p []byte) []byte { p[12] = pduStart; return p },
	} {
		if err := DecodePPICPUControlResponse(mutate(ppiCPUControlResponse(7, pduStop)), validControl); err == nil {
			t.Fatal("accepted malformed control response")
		}
	}
	if err := DecodePPICPUControlResponse(ppiCPUControlResponse(7, pduStart, pduAlreadyStarted), PPICPUControlRequest{Operation: PPICPUHotStart, PDUSize: 240, Reference: 7}); !errors.Is(err, ErrPPICPUAlreadyRunning) {
		t.Fatalf("already running error=%v", err)
	}
	if err := DecodePPICPUControlResponse(ppiCPUControlResponse(7, pduStop, pduAlreadyStopped), validControl); !errors.Is(err, ErrPPICPUAlreadyStopped) {
		t.Fatalf("already stopped error=%v", err)
	}
	validStatus := PPICPUStatusRequest{PDUSize: 240, Reference: 9}
	for _, mutate := range []func([]byte) []byte{
		func(p []byte) []byte { return p[:37] },
		func(p []byte) []byte { p[1] = 3; return p },
		func(p []byte) []byte { p[5] = 8; return p },
		func(p []byte) []byte { p[9] = 19; return p },
		func(p []byte) []byte { p[21] = 1; return p },
		func(p []byte) []byte { return append(p, 0) },
	} {
		if status, err := DecodePPICPUStatusResponse(mutate(ppiCPUStatusResponse(9, 8)), validStatus); err == nil || status != PPICPUStatusUnknown {
			t.Fatalf("accepted malformed status: %v %v", status, err)
		}
	}
}

func TestPPICPUOperationsUsePPIExchangeAndRejectBeforeIO(t *testing.T) {
	t.Parallel()
	controlResponse, _ := EncodePPIFrame(PPIFrame{Kind: PPIVariable, Destination: 0, Source: 2, Payload: ppiCPUControlResponse(7, pduStop)})
	wire := &ppiTestWire{reads: bytes.NewReader(append([]byte{byte(PPIAck)}, controlResponse...)), limit: 1}
	request := PPICPUControlRequest{Operation: PPICPUStop, PDUSize: 240, Reference: 7}
	if err := ControlPPICPU(wire, 0, 2, request); err != nil {
		t.Fatal(err)
	}
	statusResponse, _ := EncodePPIFrame(PPIFrame{Kind: PPIVariable, Destination: 0, Source: 2, Payload: ppiCPUStatusResponse(9, 8)})
	wire = &ppiTestWire{reads: bytes.NewReader(append([]byte{byte(PPIAck)}, statusResponse...)), limit: 1}
	status, err := ReadPPICPUStatus(wire, 0, 2, PPICPUStatusRequest{PDUSize: 240, Reference: 9})
	if err != nil || status != PPICPUStatusRunning {
		t.Fatalf("status=%v err=%v", status, err)
	}
	wire = &ppiTestWire{reads: bytes.NewReader(nil)}
	request.Reference = 0
	if err := ControlPPICPU(wire, 0, 2, request); err == nil || wire.writes.Len() != 0 {
		t.Fatalf("invalid request caused I/O: %v %x", err, wire.writes.Bytes())
	}
}
