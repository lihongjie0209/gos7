package gos7

import "errors"

// PPIWriteRequest describes one PPI S7 Write Var byte range. PPI uses the
// full 24-bit bit-address field, beyond the MPI2 byte-session API's bound.
type PPIWriteRequest MPI2WriteRequest

// EncodePPIWritePDU reuses the fork's S7 Write Var codec while retaining
// PPI's full bit-address range.
func EncodePPIWritePDU(request PPIWriteRequest) ([]byte, error) {
	count := len(request.Data)
	if request.Start < 0 || request.Start >= 1<<21 || count < 1 || count > 1<<21-request.Start {
		return nil, errors.New("PPI S7 write byte range exceeds bit address space")
	}
	base := MPI2WriteRequest(request)
	base.Start = 0
	pdu, err := EncodeMPI2WritePDU(base)
	if err != nil {
		return nil, err
	}
	bitStart := request.Start * 8
	pdu[21], pdu[22], pdu[23] = byte(bitStart>>16), byte(bitStart>>8), byte(bitStart)
	return pdu, nil
}

// DecodePPIWriteResponse validates one PPI Write Var acknowledgement.
func DecodePPIWriteResponse(response []byte, request PPIWriteRequest) error {
	if _, err := EncodePPIWritePDU(request); err != nil {
		return err
	}
	base := MPI2WriteRequest(request)
	base.Start = 0
	return DecodeMPI2WriteResponse(response, base)
}
