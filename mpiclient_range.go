package gos7

import (
	"errors"
	"fmt"
	"io"
)

// MPI2ReadRangeRequest describes a possibly multi-PDU byte-range read.
type MPI2ReadRangeRequest struct {
	Area          byte
	DBNumber      int
	Start         int
	Count         int
	PDUSize       int
	Reference     uint16
	MessageNumber byte
}

// MPI2WriteRangeRequest describes a possibly multi-PDU byte-range write.
type MPI2WriteRangeRequest struct {
	Area          byte
	DBNumber      int
	Start         int
	Data          []byte
	PDUSize       int
	Reference     uint16
	MessageNumber byte
}

// MPI2ReadChunk is one planned read request and its MPI2 message number.
type MPI2ReadChunk struct {
	MPI2ReadRequest
	MessageNumber byte
}

// MPI2WriteChunk is one planned write request and its MPI2 message number.
type MPI2WriteChunk struct {
	MPI2WriteRequest
	MessageNumber byte
}

// PlanMPI2ReadRange validates the whole range and all references before I/O.
func PlanMPI2ReadRange(request MPI2ReadRangeRequest) ([]MPI2ReadChunk, error) {
	if request.Count < 1 || request.Count > 1<<18 || request.Start < 0 || request.Start >= 1<<18 || request.Count > 1<<18-request.Start {
		return nil, errors.New("MPI2 S7 read range exceeds address space")
	}
	if request.MessageNumber == 0 || request.PDUSize < 64 || request.PDUSize > 240 {
		return nil, errors.New("invalid MPI2 S7 read message number or PDU size")
	}
	maxChunk := min(222, request.PDUSize-18)
	count := (request.Count + maxChunk - 1) / maxChunk
	if request.Reference == 0 || count > 65536-int(request.Reference) {
		return nil, errors.New("MPI2 S7 read references would overflow")
	}
	chunks := make([]MPI2ReadChunk, 0, count)
	message := request.MessageNumber
	for offset := 0; offset < request.Count; {
		part := MPI2ReadRequest{
			Area: request.Area, DBNumber: request.DBNumber, Start: request.Start + offset,
			Count: min(maxChunk, request.Count-offset), PDUSize: request.PDUSize,
			Reference: request.Reference + uint16(len(chunks)),
		}
		if _, err := EncodeMPI2ReadPDU(part); err != nil {
			return nil, err
		}
		chunks = append(chunks, MPI2ReadChunk{MPI2ReadRequest: part, MessageNumber: message})
		offset += part.Count
		message = nextMPI2MessageNumber(message)
	}
	return chunks, nil
}

// PlanMPI2WriteRange validates the whole range and all references before I/O.
func PlanMPI2WriteRange(request MPI2WriteRangeRequest) ([]MPI2WriteChunk, error) {
	total := len(request.Data)
	if total < 1 || total > 1<<18 || request.Start < 0 || request.Start >= 1<<18 || total > 1<<18-request.Start {
		return nil, errors.New("MPI2 S7 write range exceeds address space")
	}
	if request.MessageNumber == 0 || request.PDUSize < 64 || request.PDUSize > 240 {
		return nil, errors.New("invalid MPI2 S7 write message number or PDU size")
	}
	maxChunk := min(224, request.PDUSize-28)
	count := (total + maxChunk - 1) / maxChunk
	if request.Reference == 0 || count > 65536-int(request.Reference) {
		return nil, errors.New("MPI2 S7 write references would overflow")
	}
	chunks := make([]MPI2WriteChunk, 0, count)
	message := request.MessageNumber
	for offset := 0; offset < total; {
		length := min(maxChunk, total-offset)
		part := MPI2WriteRequest{
			Area: request.Area, DBNumber: request.DBNumber, Start: request.Start + offset,
			Data: append([]byte{}, request.Data[offset:offset+length]...), PDUSize: request.PDUSize,
			Reference: request.Reference + uint16(len(chunks)),
		}
		if _, err := EncodeMPI2WritePDU(part); err != nil {
			return nil, err
		}
		chunks = append(chunks, MPI2WriteChunk{MPI2WriteRequest: part, MessageNumber: message})
		offset += length
		message = nextMPI2MessageNumber(message)
	}
	return chunks, nil
}

func nextMPI2MessageNumber(number byte) byte {
	if number == 255 {
		return 1
	}
	return number + 1
}

// ReadMPI2Range reads a complete byte range on a caller-owned MPI2 session.
func ReadMPI2Range(wire io.ReadWriter, peer, local byte, request MPI2ReadRangeRequest) ([]byte, error) {
	chunks, err := PlanMPI2ReadRange(request)
	if err != nil {
		return nil, err
	}
	result := make([]byte, 0, request.Count)
	for _, chunk := range chunks {
		data, err := ReadMPI2Bytes(wire, peer, local, chunk.MessageNumber, chunk.MPI2ReadRequest)
		if err != nil {
			return nil, fmt.Errorf("MPI2 S7 read failed at byte %d: %w", chunk.Start, err)
		}
		result = append(result, data...)
	}
	return result, nil
}

// WriteMPI2Range writes a non-atomic byte range on a caller-owned MPI2 session.
func WriteMPI2Range(wire io.ReadWriter, peer, local byte, request MPI2WriteRangeRequest) error {
	chunks, err := PlanMPI2WriteRange(request)
	if err != nil {
		return err
	}
	var applied int
	for _, chunk := range chunks {
		if err := WriteMPI2Bytes(wire, peer, local, chunk.MessageNumber, chunk.MPI2WriteRequest); err != nil {
			return fmt.Errorf("MPI2 S7 write failed after %d bytes may have been applied: %w", applied, err)
		}
		applied += len(chunk.Data)
	}
	return nil
}
