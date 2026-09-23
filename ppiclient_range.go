package gos7

import (
	"context"
	"errors"
	"fmt"
	"io"
)

type PPIReadRangeRequest struct {
	Area      byte
	DBNumber  int
	Start     int
	Count     int
	PDUSize   int
	Reference uint16
}

type PPIWriteRangeRequest struct {
	Area      byte
	DBNumber  int
	Start     int
	Data      []byte
	PDUSize   int
	Reference uint16
}

type PPIWriteRangeError struct {
	AppliedBytes int
	Err          error
}

func (e *PPIWriteRangeError) Error() string {
	return fmt.Sprintf("PPI S7 write failed after %d bytes may have been applied: %v", e.AppliedBytes, e.Err)
}

func (e *PPIWriteRangeError) Unwrap() error { return e.Err }

func PlanPPIReadRange(request PPIReadRangeRequest) ([]PPIReadRequest, error) {
	if request.Count < 1 || request.Count > 1<<20 || request.Start < 0 || request.Start >= 1<<21 || request.Count > 1<<21-request.Start {
		return nil, errors.New("PPI S7 read range exceeds configured bounds")
	}
	if request.PDUSize < 64 || request.PDUSize > 240 {
		return nil, errors.New("invalid PPI S7 read PDU size")
	}
	maxChunk := min(222, request.PDUSize-18)
	chunkCount := (request.Count + maxChunk - 1) / maxChunk
	if request.Reference == 0 || chunkCount > 65536-int(request.Reference) {
		return nil, errors.New("PPI S7 read references would overflow")
	}
	chunks := make([]PPIReadRequest, 0, chunkCount)
	for offset := 0; offset < request.Count; {
		chunk := PPIReadRequest{
			Area: request.Area, DBNumber: request.DBNumber, Start: request.Start + offset,
			Count: min(maxChunk, request.Count-offset), PDUSize: request.PDUSize,
			Reference: request.Reference + uint16(len(chunks)),
		}
		if _, err := EncodePPIReadPDU(chunk); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
		offset += chunk.Count
	}
	return chunks, nil
}

func PlanPPIWriteRange(request PPIWriteRangeRequest) ([]PPIWriteRequest, error) {
	total := len(request.Data)
	if total < 1 || total > 1<<20 || request.Start < 0 || request.Start >= 1<<21 || total > 1<<21-request.Start {
		return nil, errors.New("PPI S7 write range exceeds configured bounds")
	}
	if request.PDUSize < 64 || request.PDUSize > 240 {
		return nil, errors.New("invalid PPI S7 write PDU size")
	}
	maxChunk := min(224, request.PDUSize-28)
	chunkCount := (total + maxChunk - 1) / maxChunk
	if request.Reference == 0 || chunkCount > 65536-int(request.Reference) {
		return nil, errors.New("PPI S7 write references would overflow")
	}
	chunks := make([]PPIWriteRequest, 0, chunkCount)
	for offset := 0; offset < total; {
		length := min(maxChunk, total-offset)
		chunk := PPIWriteRequest{
			Area: request.Area, DBNumber: request.DBNumber, Start: request.Start + offset,
			Data: append([]byte(nil), request.Data[offset:offset+length]...), PDUSize: request.PDUSize,
			Reference: request.Reference + uint16(len(chunks)),
		}
		if _, err := EncodePPIWritePDU(chunk); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
		offset += length
	}
	return chunks, nil
}

func ReadPPIRange(ctx context.Context, wire io.ReadWriter, local, target byte, request PPIReadRangeRequest) ([]byte, error) {
	if ctx == nil {
		return nil, errors.New("PPI S7 read context is required")
	}
	chunks, err := PlanPPIReadRange(request)
	if err != nil {
		return nil, err
	}
	result := make([]byte, 0, request.Count)
	for _, chunk := range chunks {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		pdu, encodeErr := EncodePPIReadPDU(chunk)
		if encodeErr != nil {
			return nil, encodeErr
		}
		response, exchangeErr := ExchangePPI(wire, local, target, pdu)
		if exchangeErr != nil {
			return nil, fmt.Errorf("PPI S7 read failed at byte %d: %w", chunk.Start, exchangeErr)
		}
		data, decodeErr := DecodePPIReadResponse(response, chunk)
		if decodeErr != nil {
			return nil, fmt.Errorf("PPI S7 read failed at byte %d: %w", chunk.Start, decodeErr)
		}
		result = append(result, data...)
	}
	return result, nil
}

func WritePPIRange(ctx context.Context, wire io.ReadWriter, local, target byte, request PPIWriteRangeRequest) error {
	if ctx == nil {
		return errors.New("PPI S7 write context is required")
	}
	chunks, err := PlanPPIWriteRange(request)
	if err != nil {
		return err
	}
	applied := 0
	for _, chunk := range chunks {
		if err = ctx.Err(); err != nil {
			return &PPIWriteRangeError{AppliedBytes: applied, Err: err}
		}
		pdu, encodeErr := EncodePPIWritePDU(chunk)
		if encodeErr != nil {
			return &PPIWriteRangeError{AppliedBytes: applied, Err: encodeErr}
		}
		response, exchangeErr := ExchangePPI(wire, local, target, pdu)
		if exchangeErr != nil {
			return &PPIWriteRangeError{AppliedBytes: applied, Err: exchangeErr}
		}
		if decodeErr := DecodePPIWriteResponse(response, chunk); decodeErr != nil {
			return &PPIWriteRangeError{AppliedBytes: applied, Err: decodeErr}
		}
		applied += len(chunk.Data)
	}
	return nil
}
