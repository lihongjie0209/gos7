package gos7

import (
	"fmt"
	"io"
)

// ExchangeMPI2PLCConnectOffer performs the first PLC connection exchange.
// Its final STX must be answered by ExchangeMPI2PLCConnectConfirm on the same wire.
func ExchangeMPI2PLCConnectOffer(wire io.ReadWriter, station, connection byte) (byte, error) {
	offer, err := EncodeMPI2PLCConnectOffer(station, connection)
	if err != nil {
		return 0, err
	}
	if wire == nil {
		return 0, io.ErrClosedPipe
	}
	frame, err := EncodeMPI2Frame(offer)
	if err != nil {
		return 0, err
	}
	if err := mpi2Write(wire, []byte{mpi2STX}); err != nil {
		return 0, fmt.Errorf("starting MPI2 PLC connection offer: %w", err)
	}
	if err := mpi2ExpectControl(wire, mpi2DLE); err != nil {
		return 0, fmt.Errorf("MPI2 PLC connection start acknowledgement: %w", err)
	}
	if err := mpi2Write(wire, frame); err != nil {
		return 0, fmt.Errorf("sending MPI2 PLC connection offer: %w", err)
	}
	if err := mpi2ExpectControl(wire, mpi2DLE); err != nil {
		return 0, fmt.Errorf("MPI2 PLC connection offer acknowledgement: %w", err)
	}
	if err := mpi2ExpectControl(wire, mpi2STX); err != nil {
		return 0, fmt.Errorf("MPI2 PLC connection response start: %w", err)
	}
	if err := mpi2Write(wire, []byte{mpi2DLE}); err != nil {
		return 0, fmt.Errorf("accepting MPI2 PLC connection response: %w", err)
	}
	response, err := ReadMPI2Frame(wire)
	if err != nil {
		return 0, fmt.Errorf("reading MPI2 PLC connection response: %w", err)
	}
	peer, err := DecodeMPI2PLCConnectAccept(response, station)
	if err != nil {
		return 0, err
	}
	if err := mpi2Write(wire, []byte{mpi2DLE, mpi2STX}); err != nil {
		return 0, fmt.Errorf("acknowledging MPI2 PLC connection response: %w", err)
	}
	return peer, nil
}

// ExchangeMPI2PLCConnectConfirm completes the pending offer on the same wire.
func ExchangeMPI2PLCConnectConfirm(wire io.ReadWriter, peer, local byte) error {
	payload, err := EncodeMPI2PLCConnectConfirm(peer, local)
	if err != nil {
		return err
	}
	if wire == nil {
		return io.ErrClosedPipe
	}
	frame, err := EncodeMPI2Frame(payload)
	if err != nil {
		return err
	}
	if err := mpi2ExpectControl(wire, mpi2DLE); err != nil {
		return fmt.Errorf("MPI2 PLC offer final acknowledgement: %w", err)
	}
	if err := mpi2Write(wire, frame); err != nil {
		return fmt.Errorf("sending MPI2 PLC confirmation: %w", err)
	}
	if err := mpi2ExpectControl(wire, mpi2DLE); err != nil {
		return fmt.Errorf("MPI2 PLC confirmation acknowledgement: %w", err)
	}
	if err := mpi2ExpectControl(wire, mpi2STX); err != nil {
		return fmt.Errorf("MPI2 PLC confirmation response start: %w", err)
	}
	if err := mpi2Write(wire, []byte{mpi2DLE}); err != nil {
		return fmt.Errorf("accepting MPI2 PLC confirmation response: %w", err)
	}
	response, err := ReadMPI2Frame(wire)
	if err != nil {
		return fmt.Errorf("reading MPI2 PLC confirmation response: %w", err)
	}
	if err := DecodeMPI2PLCConnectConfirmed(response); err != nil {
		return err
	}
	if err := mpi2Write(wire, []byte{mpi2DLE}); err != nil {
		return fmt.Errorf("acknowledging MPI2 PLC confirmation response: %w", err)
	}
	return nil
}

// ExchangeMPI2PLCConnection completes both PLC connection exchanges on one wire.
func ExchangeMPI2PLCConnection(wire io.ReadWriter, station, local byte) (byte, error) {
	peer, err := ExchangeMPI2PLCConnectOffer(wire, station, local)
	if err != nil {
		return 0, err
	}
	if err := ExchangeMPI2PLCConnectConfirm(wire, peer, local); err != nil {
		return 0, err
	}
	return peer, nil
}
