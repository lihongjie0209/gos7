package gos7

import (
	"errors"
	"time"
)

// EncodeS7DateTime strictly encodes one classic eight-byte S7 DATE_AND_TIME
// through the package's established Helper implementation.
func EncodeS7DateTime(value time.Time) ([]byte, error) {
	value = value.UTC()
	if value.Year() < 1990 || value.Year() > 2089 {
		return nil, errors.New("S7 DATE_AND_TIME year must be 1990..2089")
	}
	if value.Nanosecond()%int(time.Millisecond) != 0 {
		return nil, errors.New("S7 DATE_AND_TIME requires millisecond precision")
	}
	wire := make([]byte, 8)
	var helper Helper
	helper.SetDateTimeAt(wire, 0, value)
	return wire, nil
}

// DecodeS7DateTime strictly decodes one classic eight-byte S7 DATE_AND_TIME
// as UTC.
func DecodeS7DateTime(wire []byte) (time.Time, error) {
	if len(wire) != 8 {
		return time.Time{}, errors.New("S7 DATE_AND_TIME must contain eight bytes")
	}
	for index := 0; index < 7; index++ {
		if wire[index]>>4 > 9 || wire[index]&0x0f > 9 {
			return time.Time{}, errors.New("S7 DATE_AND_TIME contains invalid BCD")
		}
	}
	if wire[7]>>4 > 9 || wire[7]&0x0f > 6 {
		return time.Time{}, errors.New("S7 DATE_AND_TIME contains invalid millisecond or weekday")
	}
	month := decodeBcd(wire[1])
	day := decodeBcd(wire[2])
	hour := decodeBcd(wire[3])
	minute := decodeBcd(wire[4])
	second := decodeBcd(wire[5])
	if month < 1 || month > 12 || day < 1 || day > 31 || hour > 23 || minute > 59 || second > 59 {
		return time.Time{}, errors.New("S7 DATE_AND_TIME fields are out of range")
	}
	var helper Helper
	value := helper.GetDateTimeAt(wire, 0)
	if value.Year() < 1990 || value.Year() > 2089 {
		return time.Time{}, errors.New("S7 DATE_AND_TIME fields are out of range")
	}
	rebuilt := time.Date(value.Year(), value.Month(), value.Day(), value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), time.UTC)
	if int(rebuilt.Month()) != month || rebuilt.Day() != day {
		return time.Time{}, errors.New("S7 DATE_AND_TIME calendar date is invalid")
	}
	if byte(rebuilt.Weekday()) != wire[7]&0x0f {
		return time.Time{}, errors.New("S7 DATE_AND_TIME weekday is inconsistent")
	}
	return rebuilt, nil
}
