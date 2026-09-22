package gos7

import (
	"encoding/binary"
	"errors"
	"io"
)

// MPI2WriteRequest describes one byte-range S7 Write Var request.
type MPI2WriteRequest struct {
	Area      byte
	DBNumber  int
	Start     int
	Data      []byte
	PDUSize   int
	Reference uint16
}

// EncodeMPI2WritePDU builds one bounded S7 Write Var PDU from gos7's telegram.
func EncodeMPI2WritePDU(request MPI2WriteRequest) ([]byte, error) {
	switch request.Area {
	case s7areape, s7areapa, s7areamk, s7areadb:
	default:
		return nil, errors.New("unsupported MPI2 S7 byte area")
	}
	if request.DBNumber < 0 || request.DBNumber > 65535 || request.Area != s7areadb && request.DBNumber != 0 {
		return nil, errors.New("invalid MPI2 S7 DB number")
	}
	count := len(request.Data)
	if request.Start < 0 || request.Start >= 1<<18 || count < 1 || count > 224 {
		return nil, errors.New("MPI2 S7 write byte range exceeds address or frame bounds")
	}
	if count > 1<<18-request.Start {
		return nil, errors.New("MPI2 S7 write byte range exceeds bit address space")
	}
	if request.PDUSize < 64 || request.PDUSize > 240 || count > request.PDUSize-28 || request.Reference == 0 {
		return nil, errors.New("invalid MPI2 S7 write PDU size or reference")
	}
	pdu := append([]byte{}, s7ReadWriteTelegram[7:31]...)
	pdu[10] = 5
	pdu = append(pdu, 0, 4, 0, 0)
	pdu = append(pdu, request.Data...)
	binary.BigEndian.PutUint16(pdu[4:6], request.Reference)
	binary.BigEndian.PutUint16(pdu[8:10], uint16(4+count))
	binary.BigEndian.PutUint16(pdu[16:18], uint16(count))
	binary.BigEndian.PutUint16(pdu[18:20], uint16(request.DBNumber))
	pdu[20] = request.Area
	bitStart := request.Start * 8
	pdu[21], pdu[22], pdu[23] = byte(bitStart>>16), byte(bitStart>>8), byte(bitStart)
	binary.BigEndian.PutUint16(pdu[26:28], uint16(count*8))
	return pdu, nil
}

// DecodeMPI2WriteResponse validates one S7 Write Var acknowledgement.
func DecodeMPI2WriteResponse(response []byte, request MPI2WriteRequest) error {
	if _, err := EncodeMPI2WritePDU(request); err != nil {
		return err
	}
	if len(response) != 15 || response[0] != 0x32 || response[1] != 3 || response[2] != 0 || response[3] != 0 {
		return errors.New("invalid MPI2 S7 write response header or size")
	}
	if binary.BigEndian.Uint16(response[4:6]) != request.Reference || binary.BigEndian.Uint16(response[6:8]) != 2 || binary.BigEndian.Uint16(response[8:10]) != 1 {
		return errors.New("MPI2 S7 write response reference or lengths mismatch")
	}
	if response[10] != 0 || response[11] != 0 || response[12] != 5 || response[13] != 1 || response[14] != 0xff {
		return errors.New("MPI2 S7 write failed or returned wrong function")
	}
	return nil
}

// WriteMPI2Bytes exchanges one Write Var request on a caller-owned MPI2 session.
func WriteMPI2Bytes(wire io.ReadWriter, peer, local, number byte, request MPI2WriteRequest) error {
	pdu, err := EncodeMPI2WritePDU(request)
	if err != nil {
		return err
	}
	response, err := ExchangeMPI2S7PDU(wire, peer, local, number, pdu)
	if err != nil {
		return err
	}
	return DecodeMPI2WriteResponse(response, request)
}
