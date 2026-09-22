package gos7

import (
	"encoding/binary"
	"errors"
	"io"
)

// PPICPUOperation selects one established S7 CPU mode command.
type PPICPUOperation uint8

const (
	PPICPUHotStart PPICPUOperation = iota + 1
	PPICPUColdStart
	PPICPUStop
)

// PPICPUStatus is the normalized status returned by the S7 status service.
type PPICPUStatus string

const (
	PPICPUStatusUnknown PPICPUStatus = "unknown"
	PPICPUStatusRunning PPICPUStatus = "running"
	PPICPUStatusStopped PPICPUStatus = "stopped"
)

var (
	ErrPPICPUAlreadyRunning = errors.New("PPI CPU is already running")
	ErrPPICPUAlreadyStopped = errors.New("PPI CPU is already stopped")
)

type PPICPUControlRequest struct {
	Operation PPICPUOperation
	PDUSize   int
	Reference uint16
}

type PPICPUStatusRequest struct {
	PDUSize   int
	Reference uint16
}

func validatePPICPURequest(pduSize int, reference uint16, encodedSize int) error {
	if pduSize < 64 || pduSize > 240 || encodedSize > pduSize || reference == 0 {
		return errors.New("invalid PPI CPU PDU size or reference")
	}
	return nil
}

// EncodePPICPUControlPDU derives a PPI-ready PDU from gos7's established CPU
// control telegram and assigns the caller's request reference.
func EncodePPICPUControlPDU(request PPICPUControlRequest) ([]byte, error) {
	var telegram []byte
	switch request.Operation {
	case PPICPUHotStart:
		telegram = s7HotStartTelegram
	case PPICPUColdStart:
		telegram = s7ColdStartTelegram
	case PPICPUStop:
		telegram = s7StopTelegram
	default:
		return nil, errors.New("unsupported PPI CPU control operation")
	}
	pdu := append([]byte{}, telegram[7:]...)
	if err := validatePPICPURequest(request.PDUSize, request.Reference, len(pdu)); err != nil {
		return nil, err
	}
	binary.BigEndian.PutUint16(pdu[4:6], request.Reference)
	return pdu, nil
}

// DecodePPICPUControlResponse strictly validates a start or stop response.
func DecodePPICPUControlResponse(response []byte, request PPICPUControlRequest) error {
	if _, err := EncodePPICPUControlPDU(request); err != nil {
		return err
	}
	if len(response) < 13 || len(response) > request.PDUSize || response[0] != 0x32 || response[1] != 3 || response[2] != 0 || response[3] != 0 {
		return errors.New("invalid PPI CPU control response header or size")
	}
	parameterLength := int(binary.BigEndian.Uint16(response[6:8]))
	dataLength := int(binary.BigEndian.Uint16(response[8:10]))
	if binary.BigEndian.Uint16(response[4:6]) != request.Reference || len(response) != 12+parameterLength+dataLength || response[10] != 0 || response[11] != 0 {
		return errors.New("PPI CPU control response reference, lengths, or CPU error mismatch")
	}
	wantFunction := byte(pduStart)
	if request.Operation == PPICPUStop {
		wantFunction = pduStop
	}
	if parameterLength < 1 || response[12] != wantFunction {
		return errors.New("PPI CPU control returned the wrong function")
	}
	if parameterLength == 1 {
		return nil
	}
	if parameterLength != 2 || dataLength != 0 {
		return errors.New("PPI CPU control returned unexpected data")
	}
	if request.Operation == PPICPUStop && response[13] == pduAlreadyStopped {
		return ErrPPICPUAlreadyStopped
	}
	if request.Operation != PPICPUStop && response[13] == pduAlreadyStarted {
		return ErrPPICPUAlreadyRunning
	}
	return errors.New("PPI CPU control failed")
}

// EncodePPICPUStatusPDU derives a PPI-ready status PDU from gos7's established
// status telegram and assigns the caller's request reference.
func EncodePPICPUStatusPDU(request PPICPUStatusRequest) ([]byte, error) {
	pdu := append([]byte{}, s7GetStatusTelegram[7:]...)
	if err := validatePPICPURequest(request.PDUSize, request.Reference, len(pdu)); err != nil {
		return nil, err
	}
	binary.BigEndian.PutUint16(pdu[4:6], request.Reference)
	return pdu, nil
}

// DecodePPICPUStatusResponse strictly validates and normalizes CPU status.
func DecodePPICPUStatusResponse(response []byte, request PPICPUStatusRequest) (PPICPUStatus, error) {
	if _, err := EncodePPICPUStatusPDU(request); err != nil {
		return PPICPUStatusUnknown, err
	}
	if len(response) != 38 || len(response) > request.PDUSize || response[0] != 0x32 || response[1] != 7 || response[2] != 0 || response[3] != 0 {
		return PPICPUStatusUnknown, errors.New("invalid PPI CPU status response header or size")
	}
	parameterLength := int(binary.BigEndian.Uint16(response[6:8]))
	dataLength := int(binary.BigEndian.Uint16(response[8:10]))
	if binary.BigEndian.Uint16(response[4:6]) != request.Reference || len(response) != 10+parameterLength+dataLength || binary.BigEndian.Uint16(response[20:22]) != 0 {
		return PPICPUStatusUnknown, errors.New("PPI CPU status response reference, lengths, or result mismatch")
	}
	switch response[37] {
	case s7CpuStatusRun:
		return PPICPUStatusRunning, nil
	case s7CpuStatusStop, 3:
		return PPICPUStatusStopped, nil
	case s7CpuStatusUnknown:
		return PPICPUStatusUnknown, nil
	default:
		return PPICPUStatusUnknown, errors.New("PPI CPU status response contains an unknown status code")
	}
}

func ControlPPICPU(wire io.ReadWriter, local, target byte, request PPICPUControlRequest) error {
	pdu, err := EncodePPICPUControlPDU(request)
	if err != nil {
		return err
	}
	response, err := ExchangePPI(wire, local, target, pdu)
	if err != nil {
		return err
	}
	return DecodePPICPUControlResponse(response, request)
}

func ReadPPICPUStatus(wire io.ReadWriter, local, target byte, request PPICPUStatusRequest) (PPICPUStatus, error) {
	pdu, err := EncodePPICPUStatusPDU(request)
	if err != nil {
		return PPICPUStatusUnknown, err
	}
	response, err := ExchangePPI(wire, local, target, pdu)
	if err != nil {
		return PPICPUStatusUnknown, err
	}
	return DecodePPICPUStatusResponse(response, request)
}
