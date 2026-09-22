package gos7

import (
	"errors"
	"fmt"
	"io"
)

// MPI2PLCDisconnectPayload returns the PLC disconnect request payload.
func MPI2PLCDisconnectPayload(peer, local byte) ([]byte, error) {
	if local == 0 {
		return nil, errors.New("MPI connection number must be nonzero")
	}
	return []byte{0, 0x0c, peer, local, 0x80}, nil
}

// MPI2AdapterShutdownPayload returns the adapter shutdown request payload.
func MPI2AdapterShutdownPayload() []byte {
	return []byte{1, 4, 2}
}

// RequestMPI2PLCDisconnect exchanges a disconnect request and returns its
// opaque, checksum-checked reply. Reply semantics require device confirmation.
func RequestMPI2PLCDisconnect(wire io.ReadWriter, peer, local byte) ([]byte, error) {
	payload, err := MPI2PLCDisconnectPayload(peer, local)
	if err != nil {
		return nil, err
	}
	return mpi2DisconnectRequest(wire, payload, true)
}

// RequestMPI2AdapterShutdown exchanges an adapter shutdown request and returns
// its opaque, checksum-checked reply. Reply semantics require device confirmation.
func RequestMPI2AdapterShutdown(wire io.ReadWriter) ([]byte, error) {
	return mpi2DisconnectRequest(wire, MPI2AdapterShutdownPayload(), false)
}

func mpi2DisconnectRequest(wire io.ReadWriter, payload []byte, plc bool) ([]byte, error) {
	if wire == nil {
		return nil, io.ErrClosedPipe
	}
	frame, err := EncodeMPI2Frame(payload)
	if err != nil {
		return nil, err
	}
	if err := mpi2Write(wire, []byte{mpi2STX}); err != nil {
		return nil, fmt.Errorf("starting MPI2 disconnect request: %w", err)
	}
	if err := mpi2ExpectControl(wire, mpi2DLE); err != nil {
		return nil, fmt.Errorf("MPI2 disconnect start acknowledgement: %w", err)
	}
	if err := mpi2Write(wire, frame); err != nil {
		return nil, fmt.Errorf("sending MPI2 disconnect request: %w", err)
	}
	if err := mpi2ExpectControl(wire, mpi2DLE); err != nil {
		return nil, fmt.Errorf("MPI2 disconnect request acknowledgement: %w", err)
	}
	if plc {
		if err := mpi2Write(wire, []byte{mpi2DLE}); err != nil {
			return nil, fmt.Errorf("accepting MPI2 PLC disconnect reply: %w", err)
		}
	}
	if err := mpi2ExpectControl(wire, mpi2STX); err != nil {
		return nil, fmt.Errorf("MPI2 disconnect reply start: %w", err)
	}
	if !plc {
		if err := mpi2Write(wire, []byte{mpi2DLE}); err != nil {
			return nil, fmt.Errorf("accepting MPI2 adapter shutdown reply: %w", err)
		}
	}
	response, err := ReadMPI2Frame(wire)
	if err != nil {
		return nil, fmt.Errorf("reading MPI2 disconnect reply: %w", err)
	}
	if err := mpi2Write(wire, []byte{mpi2DLE}); err != nil {
		return nil, fmt.Errorf("acknowledging MPI2 disconnect reply: %w", err)
	}
	return response, nil
}
