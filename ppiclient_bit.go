package gos7

import (
	"encoding/binary"
	"errors"
	"io"
)

// PPIBitRequest identifies one bit in a PPI-accessible S7 byte area.
type PPIBitRequest struct {
	Area      byte
	DBNumber  int
	Start     int
	Bit       uint8
	PDUSize   int
	Reference uint16
}

func (request PPIBitRequest) readRequest() (PPIReadRequest, error) {
	if request.Bit > 7 {
		return PPIReadRequest{}, errors.New("invalid PPI S7 bit position")
	}
	base := PPIReadRequest{
		Area: request.Area, DBNumber: request.DBNumber, Start: request.Start,
		Count: 1, PDUSize: request.PDUSize, Reference: request.Reference,
	}
	if _, err := EncodePPIReadPDU(base); err != nil {
		return PPIReadRequest{}, err
	}
	return base, nil
}

func (request PPIBitRequest) writeRequest(value bool) (PPIWriteRequest, error) {
	if request.Bit > 7 {
		return PPIWriteRequest{}, errors.New("invalid PPI S7 bit position")
	}
	data := byte(0)
	if value {
		data = 1
	}
	base := PPIWriteRequest{
		Area: request.Area, DBNumber: request.DBNumber, Start: request.Start,
		Data: []byte{data}, PDUSize: request.PDUSize, Reference: request.Reference,
	}
	if _, err := EncodePPIWritePDU(base); err != nil {
		return PPIWriteRequest{}, err
	}
	return base, nil
}

// EncodePPIReadBitPDU builds one native S7 Read Var bit request.
func EncodePPIReadBitPDU(request PPIBitRequest) ([]byte, error) {
	base, err := request.readRequest()
	if err != nil {
		return nil, err
	}
	pdu, err := EncodePPIReadPDU(base)
	if err != nil {
		return nil, err
	}
	pdu[15] = s7wlbit
	binary.BigEndian.PutUint16(pdu[16:18], 1)
	address := request.Start*8 + int(request.Bit)
	pdu[21], pdu[22], pdu[23] = byte(address>>16), byte(address>>8), byte(address)
	return pdu, nil
}

// DecodePPIReadBitResponse validates one native S7 bit result.
func DecodePPIReadBitResponse(response []byte, request PPIBitRequest) (bool, error) {
	if _, err := EncodePPIReadBitPDU(request); err != nil {
		return false, err
	}
	if len(response) != 19 || response[0] != 0x32 || response[1] != 3 || response[2] != 0 || response[3] != 0 {
		return false, errors.New("invalid PPI S7 bit read response header or size")
	}
	if binary.BigEndian.Uint16(response[4:6]) != request.Reference || binary.BigEndian.Uint16(response[6:8]) != 2 || binary.BigEndian.Uint16(response[8:10]) != 5 {
		return false, errors.New("PPI S7 bit read response reference or lengths mismatch")
	}
	if response[10] != 0 || response[11] != 0 || response[12] != 4 || response[13] != 1 || response[14] != 0xff {
		return false, errors.New("PPI S7 bit read failed or returned wrong function")
	}
	if response[15] != tsResBit || binary.BigEndian.Uint16(response[16:18]) != 1 || response[18] > 1 {
		return false, errors.New("PPI S7 bit read returned invalid transport, length, or value")
	}
	return response[18] == 1, nil
}

// EncodePPIWriteBitPDU builds one native S7 Write Var bit request.
func EncodePPIWriteBitPDU(request PPIBitRequest, value bool) ([]byte, error) {
	base, err := request.writeRequest(value)
	if err != nil {
		return nil, err
	}
	pdu, err := EncodePPIWritePDU(base)
	if err != nil {
		return nil, err
	}
	pdu[15] = s7wlbit
	binary.BigEndian.PutUint16(pdu[16:18], 1)
	address := request.Start*8 + int(request.Bit)
	pdu[21], pdu[22], pdu[23] = byte(address>>16), byte(address>>8), byte(address)
	pdu[25] = tsResBit
	binary.BigEndian.PutUint16(pdu[26:28], 1)
	return pdu, nil
}

// DecodePPIWriteBitResponse validates one native S7 bit write acknowledgement.
func DecodePPIWriteBitResponse(response []byte, request PPIBitRequest) error {
	base, err := request.writeRequest(false)
	if err != nil {
		return err
	}
	return DecodePPIWriteResponse(response, base)
}

// ReadPPIBit exchanges one native bit request on a caller-owned PPI session.
func ReadPPIBit(wire io.ReadWriter, peer, local, number byte, request PPIBitRequest) (bool, error) {
	pdu, err := EncodePPIReadBitPDU(request)
	if err != nil {
		return false, err
	}
	response, err := ExchangeMPI2S7PDU(wire, peer, local, number, pdu)
	if err != nil {
		return false, err
	}
	return DecodePPIReadBitResponse(response, request)
}

// WritePPIBit exchanges one native bit write on a caller-owned PPI session.
func WritePPIBit(wire io.ReadWriter, peer, local, number byte, request PPIBitRequest, value bool) error {
	pdu, err := EncodePPIWriteBitPDU(request, value)
	if err != nil {
		return err
	}
	response, err := ExchangeMPI2S7PDU(wire, peer, local, number, pdu)
	if err != nil {
		return err
	}
	return DecodePPIWriteBitResponse(response, request)
}
