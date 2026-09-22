package gos7

import (
	"errors"
	"fmt"
	"io"
)

const (
	mpi2DLE        = 0x10
	mpi2ETX        = 0x03
	mpi2MaxPayload = 2048
	mpi2MaxWire    = mpi2MaxPayload*2 + 4
)

// EncodeMPI2Frame frames one MPI2 PC Adapter payload with DLE stuffing and BCC.
func EncodeMPI2Frame(payload []byte) ([]byte, error) {
	if len(payload) == 0 || len(payload) > mpi2MaxPayload {
		return nil, errors.New("MPI2 payload must be 1..2048 bytes")
	}
	wire := make([]byte, 0, len(payload)*2+3)
	bcc := byte(mpi2DLE ^ mpi2ETX)
	for _, value := range payload {
		wire = append(wire, value)
		if value == mpi2DLE {
			wire = append(wire, mpi2DLE)
		} else {
			bcc ^= value
		}
	}
	return append(wire, mpi2DLE, mpi2ETX, bcc), nil
}

// DecodeMPI2Frame validates exactly one complete framed payload.
func DecodeMPI2Frame(wire []byte) ([]byte, error) {
	if len(wire) < 4 || len(wire) > mpi2MaxWire {
		return nil, errors.New("MPI2 frame length is invalid")
	}
	payload := make([]byte, 0, min(len(wire)-3, mpi2MaxPayload))
	var bcc byte
	for index := 0; index < len(wire); {
		value := wire[index]
		if value != mpi2DLE {
			if len(payload) == mpi2MaxPayload {
				return nil, errors.New("MPI2 payload exceeds 2048 bytes")
			}
			payload = append(payload, value)
			bcc ^= value
			index++
			continue
		}
		if index+1 >= len(wire) {
			return nil, errors.New("truncated MPI2 escape")
		}
		switch wire[index+1] {
		case mpi2DLE:
			if len(payload) == mpi2MaxPayload {
				return nil, errors.New("MPI2 payload exceeds 2048 bytes")
			}
			payload = append(payload, mpi2DLE)
			index += 2
		case mpi2ETX:
			if index+3 != len(wire) || len(payload) == 0 {
				return nil, errors.New("invalid MPI2 frame trailer")
			}
			bcc ^= mpi2DLE ^ mpi2ETX
			if bcc != wire[index+2] {
				return nil, errors.New("MPI2 frame BCC mismatch")
			}
			return payload, nil
		default:
			return nil, errors.New("invalid MPI2 DLE escape")
		}
	}
	return nil, errors.New("MPI2 frame has no terminator")
}

// ReadMPI2Frame reads one frame without consuming bytes from the next frame.
func ReadMPI2Frame(reader io.Reader) ([]byte, error) {
	if reader == nil {
		return nil, errors.New("MPI2 reader is required")
	}
	wire := make([]byte, 0, 128)
	var one [1]byte
	previousDLE := false
	for len(wire) < mpi2MaxWire {
		if _, err := io.ReadFull(reader, one[:]); err != nil {
			return nil, fmt.Errorf("reading MPI2 frame: %w", err)
		}
		wire = append(wire, one[0])
		if previousDLE && one[0] == mpi2ETX {
			if len(wire) >= mpi2MaxWire {
				return nil, errors.New("MPI2 frame exceeds wire bound")
			}
			if _, err := io.ReadFull(reader, one[:]); err != nil {
				return nil, fmt.Errorf("reading MPI2 BCC: %w", err)
			}
			wire = append(wire, one[0])
			return DecodeMPI2Frame(wire)
		}
		if previousDLE && one[0] != mpi2DLE {
			return nil, errors.New("invalid MPI2 DLE escape")
		}
		previousDLE = !previousDLE && one[0] == mpi2DLE
	}
	return nil, errors.New("MPI2 frame exceeds wire bound")
}
