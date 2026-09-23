package gos7

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"testing"
)

func TestPPIRangePlansCompleteOperationBeforeIO(t *testing.T) {
	reads, err := PlanPPIReadRange(PPIReadRangeRequest{
		Area: 0x84, DBNumber: 1, Start: 10, Count: 47, PDUSize: 64, Reference: 2,
	})
	if err != nil || len(reads) != 2 || reads[0].Count != 46 || reads[1].Start != 56 || reads[1].Reference != 3 {
		t.Fatalf("reads=%+v err=%v", reads, err)
	}
	writes, err := PlanPPIWriteRange(PPIWriteRangeRequest{
		Area: 0x83, Start: 20, Data: bytes.Repeat([]byte{1}, 37), PDUSize: 64, Reference: 4,
	})
	if err != nil || len(writes) != 2 || len(writes[0].Data) != 36 || writes[1].Start != 56 || writes[1].Reference != 5 {
		t.Fatalf("writes=%+v err=%v", writes, err)
	}
	writes[0].Data[0] = 9
	if writes[1].Data[0] != 1 {
		t.Fatal("planned chunks alias each other")
	}
	bad := &ppiTestWire{reads: bytes.NewReader(nil)}
	if _, err = ReadPPIRange(context.Background(), bad, 0, 2, PPIReadRangeRequest{Area: 0x83, Count: 47, PDUSize: 64, Reference: 65535}); err == nil || bad.writes.Len() != 0 {
		t.Fatalf("invalid range caused I/O: err=%v writes=%x", err, bad.writes.Bytes())
	}
}

func TestPPIRangeReadAndTypedWritePrefixError(t *testing.T) {
	first := bytes.Repeat([]byte{0x11}, 46)
	second := []byte{0x22}
	readWire := &ppiTestWire{reads: bytes.NewReader(ppiRangeReplies(t,
		ppiRangeReadResponse(2, first), ppiRangeReadResponse(3, second))), limit: 1}
	got, err := ReadPPIRange(context.Background(), readWire, 0, 2, PPIReadRangeRequest{
		Area: 0x83, Start: 10, Count: 47, PDUSize: 64, Reference: 2,
	})
	if err != nil || !bytes.Equal(got, append(first, second...)) {
		t.Fatalf("read=%x err=%v", got, err)
	}

	writeWire := &ppiTestWire{reads: bytes.NewReader(ppiRangeReplies(t,
		ppiRangeWriteResponse(4, 0xff), ppiRangeWriteResponse(5, 5))), limit: 1}
	err = WritePPIRange(context.Background(), writeWire, 0, 2, PPIWriteRangeRequest{
		Area: 0x83, Start: 20, Data: bytes.Repeat([]byte{1}, 37), PDUSize: 64, Reference: 4,
	})
	var partial *PPIWriteRangeError
	if !errors.As(err, &partial) || partial.AppliedBytes != 36 || partial.Unwrap() == nil {
		t.Fatalf("write error=%#v", err)
	}
}

func TestPPIRangeHonorsCancellationBeforeExchange(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	wire := &ppiTestWire{reads: bytes.NewReader(nil)}
	_, err := ReadPPIRange(ctx, wire, 0, 2, PPIReadRangeRequest{Area: 0x83, Count: 1, PDUSize: 64, Reference: 1})
	if !errors.Is(err, context.Canceled) || wire.writes.Len() != 0 {
		t.Fatalf("read err=%v writes=%x", err, wire.writes.Bytes())
	}
}

func ppiRangeReplies(t *testing.T, pdus ...[]byte) []byte {
	t.Helper()
	var result []byte
	for _, pdu := range pdus {
		frame, err := EncodePPIFrame(PPIFrame{Kind: PPIVariable, Destination: 0, Source: 2, Control: 8, Payload: pdu})
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, byte(PPIAck))
		result = append(result, frame...)
	}
	return result
}

func ppiRangeReadResponse(reference uint16, data []byte) []byte {
	response := make([]byte, 18+len(data))
	response[0], response[1], response[4] = 0x32, 3, byte(reference>>8)
	response[5] = byte(reference)
	binary.BigEndian.PutUint16(response[6:8], 2)
	response[12], response[13], response[14], response[15] = 4, 1, 0xff, 4
	binary.BigEndian.PutUint16(response[16:18], uint16(len(data)*8))
	copy(response[18:], data)
	if len(data)%2 != 0 {
		response = append(response, 0)
	}
	binary.BigEndian.PutUint16(response[8:10], uint16(len(response)-14))
	return response
}

func ppiRangeWriteResponse(reference uint16, status byte) []byte {
	return []byte{0x32, 3, 0, 0, byte(reference >> 8), byte(reference), 0, 2, 0, 1, 0, 0, 5, 1, status}
}
