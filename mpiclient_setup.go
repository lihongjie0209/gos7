package gos7

import (
	"encoding/binary"
	"errors"
	"io"
)

// EncodeMPI2SetupRequest returns an owned serial S7 Setup PDU using the
// existing S7 Setup telegram's parameter layout without its ISO-on-TCP header.
func EncodeMPI2SetupRequest() []byte {
	pdu := append([]byte{}, s7PDUNegogiationTelegram[7:]...)
	binary.BigEndian.PutUint16(pdu[4:6], 1)
	binary.BigEndian.PutUint16(pdu[16:18], 240)
	return pdu
}

// DecodeMPI2SetupResponse validates an S7 Setup Ack_Data reply for MPI2.
func DecodeMPI2SetupResponse(pdu []byte) (int, error) {
	if len(pdu) != 20 || pdu[0] != 0x32 || pdu[1] != 3 || pdu[2] != 0 || pdu[3] != 0 {
		return 0, errors.New("invalid MPI2 S7 setup response header")
	}
	if binary.BigEndian.Uint16(pdu[4:6]) != 1 || binary.BigEndian.Uint16(pdu[6:8]) != 8 || binary.BigEndian.Uint16(pdu[8:10]) != 0 {
		return 0, errors.New("MPI2 S7 setup response reference or lengths mismatch")
	}
	if pdu[10] != 0 || pdu[11] != 0 || pdu[12] != 0xf0 || pdu[13] != 0 {
		return 0, errors.New("MPI2 S7 setup rejected or unexpected parameter")
	}
	if binary.BigEndian.Uint16(pdu[14:16]) == 0 || binary.BigEndian.Uint16(pdu[16:18]) == 0 {
		return 0, errors.New("MPI2 S7 setup outstanding-job count is zero")
	}
	size := int(binary.BigEndian.Uint16(pdu[18:20]))
	if size < 64 || size > 240 {
		return 0, errors.New("MPI2 S7 negotiated PDU size is outside serial bounds")
	}
	return size, nil
}

// NegotiateMPI2PDU exchanges one S7 Setup PDU on a caller-owned connected session.
func NegotiateMPI2PDU(wire io.ReadWriter, peer, local, requestNumber byte) (int, error) {
	response, err := ExchangeMPI2S7PDU(wire, peer, local, requestNumber, EncodeMPI2SetupRequest())
	if err != nil {
		return 0, err
	}
	return DecodeMPI2SetupResponse(response)
}
