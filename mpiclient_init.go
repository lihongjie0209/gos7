package gos7

import (
	"errors"
	"fmt"
	"io"
)

const mpi2STX = 0x02

// MPI2AdapterInitPayload encodes the 23-byte serial PC Adapter configuration.
func MPI2AdapterInitPayload(localStation byte, busBaud int) ([]byte, error) {
	if localStation > 126 {
		return nil, errors.New("MPI local station must be 0..126")
	}
	var speedCode byte
	timing := byte(0x3c)
	switch busBaud {
	case 9600:
		speedCode = 0
	case 19200:
		speedCode = 1
	case 187500:
		speedCode = 2
	case 500000:
		speedCode, timing = 3, 0x64
	case 1500000:
		speedCode, timing = 4, 0x96
	case 45450:
		speedCode = 5
	case 93750:
		speedCode = 6
	default:
		return nil, fmt.Errorf("unsupported MPI bus speed %d", busBaud)
	}
	return []byte{
		0x01, 0x03, 0x02, 0x17, 0x00, 0x9f, 0x01, timing,
		0x00, 0x90, 0x01, 0x14, 0x00, 0x00, 0x05, speedCode,
		localStation, 0x0f, 0x05, 0x01, 0x01, 0x03, 0x80,
	}, nil
}

// InitializeMPI2Adapter configures a caller-owned, deadline-bounded serial session.
func InitializeMPI2Adapter(wire io.ReadWriter, localStation byte, busBaud int) ([]byte, error) {
	payload, err := MPI2AdapterInitPayload(localStation, busBaud)
	if err != nil {
		return nil, err
	}
	if wire == nil {
		return nil, io.ErrClosedPipe
	}
	if err := mpi2Write(wire, []byte{mpi2STX}); err != nil {
		return nil, fmt.Errorf("starting MPI2 adapter initialization: %w", err)
	}
	if err := mpi2ExpectControl(wire, mpi2DLE); err != nil {
		return nil, fmt.Errorf("MPI2 adapter start acknowledgement: %w", err)
	}
	frame, err := EncodeMPI2Frame(payload)
	if err != nil {
		return nil, err
	}
	if err := mpi2Write(wire, frame); err != nil {
		return nil, fmt.Errorf("sending MPI2 adapter configuration: %w", err)
	}
	if err := mpi2ExpectControl(wire, mpi2DLE); err != nil {
		return nil, fmt.Errorf("MPI2 adapter configuration acknowledgement: %w", err)
	}
	if err := mpi2ExpectControl(wire, mpi2STX); err != nil {
		return nil, fmt.Errorf("MPI2 adapter response start: %w", err)
	}
	if err := mpi2Write(wire, []byte{mpi2DLE}); err != nil {
		return nil, fmt.Errorf("accepting MPI2 adapter response: %w", err)
	}
	response, err := ReadMPI2Frame(wire)
	if err != nil {
		return nil, fmt.Errorf("reading MPI2 adapter response: %w", err)
	}
	if err := mpi2Write(wire, []byte{mpi2DLE}); err != nil {
		return nil, fmt.Errorf("acknowledging MPI2 adapter response: %w", err)
	}
	return response, nil
}

func mpi2ExpectControl(reader io.Reader, want byte) error {
	var control [1]byte
	if _, err := io.ReadFull(reader, control[:]); err != nil {
		return err
	}
	if control[0] != want {
		return fmt.Errorf("unexpected MPI adapter control byte %02x, want %02x", control[0], want)
	}
	return nil
}

func mpi2Write(writer io.Writer, data []byte) error {
	n, err := writer.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return nil
}
