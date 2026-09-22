package gos7

import (
	"errors"
	"fmt"
)

// EncodeMPI2PLCConnectOffer encodes the first PLC connection request.
func EncodeMPI2PLCConnectOffer(station, connection byte) ([]byte, error) {
	if station > 126 {
		return nil, errors.New("MPI target station must be 0..126")
	}
	if connection == 0 {
		return nil, errors.New("MPI connection number must be nonzero")
	}
	return []byte{
		0x00, 0x0d, 0x00, connection, 0xe0, 0x04, 0x00, 0x80,
		0x00, 0x02, 0x01, 0x06, 0x01, 0x00, 0x00, 0x01,
		0x02, station, 0x01, 0x00,
	}, nil
}

// DecodeMPI2PLCConnectAccept validates the first PLC connection reply.
func DecodeMPI2PLCConnectAccept(response []byte, station byte) (byte, error) {
	if station > 126 {
		return 0, errors.New("MPI target station must be 0..126")
	}
	pattern := [...]byte{
		0x00, 0x0c, 0, 0, 0xd0, 0x04, 0x00, 0x80,
		0x01, 0x06, 0x00, 0x02, 0x00, 0x01, 0x02, station,
		0x01, 0x00, 0x01, 0x00,
	}
	if len(response) != len(pattern) {
		return 0, fmt.Errorf("MPI2 PLC connection reply has %d bytes, want %d", len(response), len(pattern))
	}
	for index, want := range pattern {
		if index == 2 || index == 3 {
			continue
		}
		if response[index] != want {
			return 0, fmt.Errorf("MPI2 PLC connection reply byte %d is %02x, want %02x", index, response[index], want)
		}
	}
	return response[3], nil
}

// EncodeMPI2PLCConnectConfirm encodes the second PLC connection request.
func EncodeMPI2PLCConnectConfirm(peer, local byte) ([]byte, error) {
	if local == 0 {
		return nil, errors.New("MPI connection number must be nonzero")
	}
	return []byte{0, 0x0c, peer, local, 5, 1}, nil
}

// DecodeMPI2PLCConnectConfirmed validates the second PLC connection reply.
func DecodeMPI2PLCConnectConfirmed(payload []byte) error {
	if len(payload) != 6 {
		return fmt.Errorf("MPI2 PLC confirmation has %d bytes, want 6", len(payload))
	}
	for _, field := range []struct {
		index int
		want  byte
	}{
		{index: 0, want: 0},
		{index: 1, want: 0x0c},
		{index: 4, want: 5},
		{index: 5, want: 1},
	} {
		if payload[field.index] != field.want {
			return fmt.Errorf("MPI2 PLC confirmation byte %d is %02x, want %02x", field.index, payload[field.index], field.want)
		}
	}
	return nil
}
