package gos7

import (
	"fmt"
	"io"
)

// ExchangeMPI2S7PDU runs one request/acknowledgement/response dialogue on a
// caller-owned, deadline-bounded MPI2 session. It does not advance numbers.
func ExchangeMPI2S7PDU(wire io.ReadWriter, peer, local, requestNumber byte, pdu []byte) ([]byte, error) {
	payload, err := EncodeMPI2PDUEnvelope(peer, local, requestNumber, pdu)
	if err != nil {
		return nil, err
	}
	if wire == nil {
		return nil, io.ErrClosedPipe
	}
	frame, err := EncodeMPI2Frame(payload)
	if err != nil {
		return nil, err
	}
	if err := mpi2Write(wire, []byte{mpi2STX}); err != nil {
		return nil, fmt.Errorf("starting MPI2 S7 PDU exchange: %w", err)
	}
	if err := mpi2ExpectControl(wire, mpi2DLE); err != nil {
		return nil, fmt.Errorf("MPI2 S7 request start acknowledgement: %w", err)
	}
	if err := mpi2Write(wire, frame); err != nil {
		return nil, fmt.Errorf("sending MPI2 S7 request: %w", err)
	}
	if err := mpi2ExpectControl(wire, mpi2DLE); err != nil {
		return nil, fmt.Errorf("MPI2 S7 request frame acknowledgement: %w", err)
	}
	if err := mpi2ExpectControl(wire, mpi2STX); err != nil {
		return nil, fmt.Errorf("MPI2 S7 acknowledgement start: %w", err)
	}
	if err := mpi2Write(wire, []byte{mpi2DLE}); err != nil {
		return nil, fmt.Errorf("accepting MPI2 S7 acknowledgement: %w", err)
	}
	ack, err := ReadMPI2Frame(wire)
	if err != nil {
		return nil, fmt.Errorf("reading MPI2 S7 acknowledgement: %w", err)
	}
	if err := DecodeMPI2MessageAck(ack, peer, local, requestNumber); err != nil {
		return nil, err
	}
	if err := mpi2Write(wire, []byte{mpi2DLE}); err != nil {
		return nil, fmt.Errorf("acknowledging MPI2 S7 request acknowledgement: %w", err)
	}
	if err := mpi2ExpectControl(wire, mpi2STX); err != nil {
		return nil, fmt.Errorf("MPI2 S7 response start: %w", err)
	}
	if err := mpi2Write(wire, []byte{mpi2DLE}); err != nil {
		return nil, fmt.Errorf("accepting MPI2 S7 response: %w", err)
	}
	responseFrame, err := ReadMPI2Frame(wire)
	if err != nil {
		return nil, fmt.Errorf("reading MPI2 S7 response: %w", err)
	}
	responseNumber, response, err := DecodeMPI2PDUEnvelope(responseFrame, peer, local)
	if err != nil {
		return nil, err
	}
	responseAck, err := EncodeMPI2MessageAck(peer, local, responseNumber)
	if err != nil {
		return nil, err
	}
	responseAckFrame, err := EncodeMPI2Frame(responseAck)
	if err != nil {
		return nil, err
	}
	if err := mpi2Write(wire, []byte{mpi2DLE, mpi2STX}); err != nil {
		return nil, fmt.Errorf("accepting MPI2 S7 response frame: %w", err)
	}
	if err := mpi2ExpectControl(wire, mpi2DLE); err != nil {
		return nil, fmt.Errorf("MPI2 S7 response final acknowledgement: %w", err)
	}
	if err := mpi2Write(wire, responseAckFrame); err != nil {
		return nil, fmt.Errorf("sending MPI2 S7 response acknowledgement: %w", err)
	}
	if err := mpi2ExpectControl(wire, mpi2DLE); err != nil {
		return nil, fmt.Errorf("MPI2 S7 response acknowledgement receipt: %w", err)
	}
	return response, nil
}
