package gos7

import "errors"

// PPIReadRequest describes one PPI S7 Read Var byte range. PPI uses the full
// 24-bit bit-address field; the MPI2 byte-session API has a narrower bound.
type PPIReadRequest MPI2ReadRequest

// EncodePPIReadPDU reuses the fork's S7 Read Var codec while retaining PPI's
// full bit-address range.
func EncodePPIReadPDU(request PPIReadRequest) ([]byte, error) {
	if request.Start < 0 || request.Start >= 1<<21 || request.Count < 1 || request.Count > 1<<21-request.Start {
		return nil, errors.New("PPI S7 read byte range exceeds bit address space")
	}
	base := MPI2ReadRequest(request)
	base.Start = 0
	pdu, err := EncodeMPI2ReadPDU(base)
	if err != nil {
		return nil, err
	}
	bitStart := request.Start * 8
	pdu[21], pdu[22], pdu[23] = byte(bitStart>>16), byte(bitStart>>8), byte(bitStart)
	return pdu, nil
}

// DecodePPIReadResponse validates a PPI Read Var reply and copies its data.
func DecodePPIReadResponse(response []byte, request PPIReadRequest) ([]byte, error) {
	if _, err := EncodePPIReadPDU(request); err != nil {
		return nil, err
	}
	base := MPI2ReadRequest(request)
	base.Start = 0
	return DecodeMPI2ReadResponse(response, base)
}
