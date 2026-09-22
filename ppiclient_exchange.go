package gos7

import (
	"errors"
	"io"
)

// ReadPPIFrame consumes exactly one framed message from a byte stream.
func ReadPPIFrame(reader io.Reader) ([]byte, error) {
	if reader == nil {
		return nil, errors.New("PPI reader is required")
	}
	var start [1]byte
	if _, err := io.ReadFull(reader, start[:]); err != nil {
		return nil, err
	}
	wire := []byte{start[0]}
	remaining := 0
	switch PPIFrameKind(start[0]) {
	case PPIAck:
		return wire, nil
	case PPIFixed:
		remaining = 5
	case PPIVariable:
		var header [3]byte
		if _, err := io.ReadFull(reader, header[:]); err != nil {
			return nil, err
		}
		wire = append(wire, header[:]...)
		if header[0] < 3 || header[0] != header[1] || header[2] != 0x68 {
			return nil, errors.New("invalid PPI variable frame header")
		}
		remaining = int(header[0]) + 2
	default:
		return nil, errors.New("unknown PPI frame start")
	}
	tail := make([]byte, remaining)
	if _, err := io.ReadFull(reader, tail); err != nil {
		return nil, err
	}
	wire = append(wire, tail...)
	if _, err := DecodePPIFrame(wire); err != nil {
		return nil, err
	}
	return wire, nil
}

// ExchangePPI performs one bounded request, acknowledgement and response
// dialogue on a caller-owned, deadline-bound exclusive serial session.
// It never retries the original request after an ambiguous I/O failure.
func ExchangePPI(wire io.ReadWriter, local, target byte, payload []byte) ([]byte, error) {
	if wire == nil || local > 126 || target > 126 || local == target || len(payload) == 0 {
		return nil, errors.New("PPI exchange requires distinct valid stations, wire and payload")
	}
	request, err := EncodePPIFrame(PPIFrame{
		Kind: PPIVariable, Destination: target, Source: local, Control: 0x6c, Payload: payload,
	})
	if err != nil {
		return nil, err
	}
	if err := writePPIFrame(wire, request); err != nil {
		return nil, err
	}
	ack, err := ReadPPIFrame(wire)
	if err != nil {
		return nil, err
	}
	if len(ack) != 1 || ack[0] != byte(PPIAck) {
		return nil, errors.New("PPI request was not acknowledged")
	}
	for poll := 0; poll < 5; poll++ {
		control := byte(0x5c)
		if poll%2 != 0 {
			control = 0x7c
		}
		fetch, err := EncodePPIFrame(PPIFrame{
			Kind: PPIFixed, Destination: target, Source: local, Control: control,
		})
		if err != nil {
			return nil, err
		}
		if err := writePPIFrame(wire, fetch); err != nil {
			return nil, err
		}
		responseWire, err := ReadPPIFrame(wire)
		if err != nil {
			return nil, err
		}
		response, err := DecodePPIFrame(responseWire)
		if err != nil {
			return nil, err
		}
		if response.Kind == PPIAck {
			continue
		}
		if response.Kind != PPIVariable || response.Destination != local || response.Source != target || len(response.Payload) == 0 {
			return nil, errors.New("invalid PPI response frame")
		}
		return response.Payload, nil
	}
	return nil, errors.New("PPI response not ready after five polls")
}

func writePPIFrame(writer io.Writer, frame []byte) error {
	n, err := writer.Write(frame)
	if err != nil {
		return err
	}
	if n != len(frame) {
		return io.ErrShortWrite
	}
	return nil
}
