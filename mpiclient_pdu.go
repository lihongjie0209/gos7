package gos7

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const mpi2PDUEnvelopeSize = 6

// EncodeMPI2PDUEnvelope wraps one valid S7 PDU for an MPI2 PC Adapter.
func EncodeMPI2PDUEnvelope(peer, local, number byte, pdu []byte) ([]byte, error) {
	if local == 0 {
		return nil, errors.New("MPI connection number must be nonzero")
	}
	if len(pdu) > mpi2MaxPayload-mpi2PDUEnvelopeSize {
		return nil, errors.New("MPI2 S7 PDU exceeds adapter payload bound")
	}
	if err := validateMPI2S7PDU(pdu); err != nil {
		return nil, err
	}
	payload := make([]byte, mpi2PDUEnvelopeSize+len(pdu))
	copy(payload, []byte{0, 0x0c, peer, local, 0xf1, number})
	copy(payload[mpi2PDUEnvelopeSize:], pdu)
	return payload, nil
}

// DecodeMPI2PDUEnvelope validates an MPI2 payload and returns an owned S7 PDU.
func DecodeMPI2PDUEnvelope(payload []byte, peer, local byte) (byte, []byte, error) {
	if local == 0 || len(payload) < mpi2PDUEnvelopeSize+10 || len(payload) > mpi2MaxPayload {
		return 0, nil, errors.New("MPI2 S7 envelope size or local connection is invalid")
	}
	if payload[0] != 0 || payload[1] != 0x0c || payload[2] != peer || payload[3] != local || payload[4] != 0xf1 {
		return 0, nil, errors.New("MPI2 S7 envelope connection or marker mismatch")
	}
	pdu := payload[mpi2PDUEnvelopeSize:]
	if err := validateMPI2S7PDU(pdu); err != nil {
		return 0, nil, err
	}
	return payload[5], append([]byte{}, pdu...), nil
}

func validateMPI2S7PDU(pdu []byte) error {
	if len(pdu) < 10 || pdu[0] != 0x32 {
		return errors.New("MPI2 S7 PDU header is invalid")
	}
	headerSize := 10
	switch pdu[1] {
	case 1:
	case 2, 3:
		headerSize = 12
	default:
		return fmt.Errorf("unsupported MPI2 S7 PDU type %d", pdu[1])
	}
	if len(pdu) < headerSize {
		return errors.New("MPI2 S7 PDU header is truncated")
	}
	paramSize := int(binary.BigEndian.Uint16(pdu[6:8]))
	dataSize := int(binary.BigEndian.Uint16(pdu[8:10]))
	if headerSize+paramSize+dataSize != len(pdu) {
		return errors.New("MPI2 S7 PDU parameter or data length mismatch")
	}
	return nil
}

// EncodeMPI2MessageAck encodes one numbered MPI2 message acknowledgement.
func EncodeMPI2MessageAck(peer, local, number byte) ([]byte, error) {
	if local == 0 {
		return nil, errors.New("MPI connection number must be nonzero")
	}
	return []byte{0, 0x0c, peer, local, 0xb0, 1, number}, nil
}

// DecodeMPI2MessageAck validates the expected numbered acknowledgement.
func DecodeMPI2MessageAck(payload []byte, peer, local, number byte) error {
	if local == 0 || len(payload) != 7 {
		return errors.New("MPI2 acknowledgement size or local connection is invalid")
	}
	want := [...]byte{0, 0x0c, peer, local, 0xb0, 1, number}
	for index, value := range want {
		if payload[index] != value {
			return fmt.Errorf("MPI2 acknowledgement byte %d is %02x, want %02x", index, payload[index], value)
		}
	}
	return nil
}
