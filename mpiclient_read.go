package gos7

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// MPI2ReadRequest describes one byte-range S7 Read Var request.
type MPI2ReadRequest struct {
	Area      byte
	DBNumber  int
	Start     int
	Count     int
	PDUSize   int
	Reference uint16
}

// EncodeMPI2ReadPDU builds one bounded S7 Read Var PDU from gos7's telegram.
func EncodeMPI2ReadPDU(request MPI2ReadRequest) ([]byte, error) {
	switch request.Area {
	case s7areape, s7areapa, s7areamk, s7areadb:
	default:
		return nil, errors.New("unsupported MPI2 S7 byte area")
	}
	if request.DBNumber < 0 || request.DBNumber > 65535 || request.Area != s7areadb && request.DBNumber != 0 {
		return nil, errors.New("invalid MPI2 S7 DB number")
	}
	if request.Start < 0 || request.Start >= 1<<18 || request.Count < 1 || request.Count > 222 {
		return nil, errors.New("MPI2 S7 read byte range exceeds address or frame bounds")
	}
	if request.Count > 1<<18-request.Start {
		return nil, errors.New("MPI2 S7 read byte range exceeds bit address space")
	}
	if request.PDUSize < 64 || request.PDUSize > 240 || request.Count > request.PDUSize-18 || request.Reference == 0 {
		return nil, errors.New("invalid MPI2 S7 read PDU size or reference")
	}
	pdu := append([]byte{}, s7ReadWriteTelegram[7:31]...)
	binary.BigEndian.PutUint16(pdu[4:6], request.Reference)
	binary.BigEndian.PutUint16(pdu[16:18], uint16(request.Count))
	binary.BigEndian.PutUint16(pdu[18:20], uint16(request.DBNumber))
	pdu[20] = request.Area
	bitStart := request.Start * 8
	pdu[21], pdu[22], pdu[23] = byte(bitStart>>16), byte(bitStart>>8), byte(bitStart)
	return pdu, nil
}

// DecodeMPI2ReadResponse validates and copies one S7 Read Var byte result.
func DecodeMPI2ReadResponse(response []byte, request MPI2ReadRequest) ([]byte, error) {
	if _, err := EncodeMPI2ReadPDU(request); err != nil {
		return nil, err
	}
	if len(response) < 18 || len(response) > request.PDUSize {
		return nil, errors.New("invalid MPI2 S7 read response size")
	}
	if response[0] != 0x32 || response[1] != 3 || response[2] != 0 || response[3] != 0 {
		return nil, errors.New("invalid MPI2 S7 read response header")
	}
	if binary.BigEndian.Uint16(response[4:6]) != request.Reference || binary.BigEndian.Uint16(response[6:8]) != 2 {
		return nil, errors.New("MPI2 S7 read response reference or parameter length mismatch")
	}
	if binary.BigEndian.Uint16(response[8:10]) != uint16(len(response)-14) {
		return nil, errors.New("MPI2 S7 read response data length mismatch")
	}
	if response[10] != 0 || response[11] != 0 || response[12] != 4 || response[13] != 1 {
		return nil, errors.New("MPI2 S7 read response error or wrong function")
	}
	if response[14] != 0xff || response[15] != 4 || binary.BigEndian.Uint16(response[16:18]) != uint16(request.Count*8) {
		return nil, fmt.Errorf("MPI2 S7 read item failed or has wrong byte length: %x", response[14:18])
	}
	dataEnd := 18 + request.Count
	if len(response) != dataEnd {
		validPadding := request.Count%2 == 1 && len(response) == dataEnd+1 && response[dataEnd] == 0
		if !validPadding {
			return nil, errors.New("MPI2 S7 read response has unexpected trailing data")
		}
	}
	return append([]byte{}, response[18:dataEnd]...), nil
}

// ReadMPI2Bytes exchanges one Read Var request on a caller-owned MPI2 session.
func ReadMPI2Bytes(wire io.ReadWriter, peer, local, number byte, request MPI2ReadRequest) ([]byte, error) {
	pdu, err := EncodeMPI2ReadPDU(request)
	if err != nil {
		return nil, err
	}
	response, err := ExchangeMPI2S7PDU(wire, peer, local, number, pdu)
	if err != nil {
		return nil, err
	}
	return DecodeMPI2ReadResponse(response, request)
}
