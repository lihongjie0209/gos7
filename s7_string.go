package gos7

import (
	"errors"
	"unicode/utf8"
)

// EncodeS7String strictly encodes one fixed-capacity classic S7 STRING using
// the package's established Helper implementation. The value is never
// truncated.
func EncodeS7String(maximum int, value string) ([]byte, error) {
	if maximum < 1 || maximum > 254 {
		return nil, errors.New("S7 STRING maximum length must be 1..254")
	}
	if !utf8.ValidString(value) {
		return nil, errors.New("S7 STRING value must be valid UTF-8")
	}
	if len(value) > maximum {
		return nil, errors.New("S7 STRING value exceeds maximum byte length")
	}
	wire := make([]byte, maximum+2)
	var helper Helper
	helper.SetStringAt(wire, 0, maximum, value)
	return wire, nil
}

// DecodeS7String strictly decodes one fixed-capacity classic S7 STRING using
// the package's established Helper implementation.
func DecodeS7String(wire []byte, expectedMaximum int) (string, error) {
	if expectedMaximum < 1 || expectedMaximum > 254 || len(wire) != expectedMaximum+2 {
		return "", errors.New("invalid S7 STRING fixed length")
	}
	if int(wire[0]) != expectedMaximum {
		return "", errors.New("S7 STRING maximum-length header mismatch")
	}
	current := int(wire[1])
	if current > expectedMaximum {
		return "", errors.New("S7 STRING current length exceeds maximum")
	}
	if !utf8.Valid(wire[2 : 2+current]) {
		return "", errors.New("S7 STRING content must be valid UTF-8")
	}
	for _, value := range wire[2+current:] {
		if value != 0 {
			return "", errors.New("S7 STRING padding must be zero")
		}
	}
	var helper Helper
	return helper.GetStringAt(wire, 0), nil
}
