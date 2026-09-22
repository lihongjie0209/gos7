package gos7

import "errors"

// PPIFrameKind identifies the serial PPI frame layout.
type PPIFrameKind byte

const (
	PPIAck      PPIFrameKind = 0xe5
	PPIFixed    PPIFrameKind = 0x10
	PPIVariable PPIFrameKind = 0x68
)

// PPIFrame is one complete PPI frame. An acknowledgement has no other fields.
type PPIFrame struct {
	Kind        PPIFrameKind
	Destination byte
	Source      byte
	Control     byte
	Payload     []byte
}

// EncodePPIFrame returns one complete frame without writing partial output.
func EncodePPIFrame(frame PPIFrame) ([]byte, error) {
	if frame.Kind == PPIAck {
		if frame.Destination != 0 || frame.Source != 0 || frame.Control != 0 || len(frame.Payload) != 0 {
			return nil, errors.New("PPI acknowledgement has no fields")
		}
		return []byte{byte(PPIAck)}, nil
	}
	if frame.Destination > 126 || frame.Source > 126 {
		return nil, errors.New("PPI address must be 0..126")
	}
	switch frame.Kind {
	case PPIFixed:
		if len(frame.Payload) != 0 {
			return nil, errors.New("PPI fixed frame has no payload")
		}
		return []byte{0x10, frame.Destination, frame.Source, frame.Control,
			ppiChecksum([]byte{frame.Destination, frame.Source, frame.Control}), 0x16}, nil
	case PPIVariable:
		if len(frame.Payload) > 252 {
			return nil, errors.New("PPI variable frame payload exceeds 252 bytes")
		}
		length := byte(3 + len(frame.Payload))
		wire := make([]byte, 0, len(frame.Payload)+9)
		wire = append(wire, 0x68, length, length, 0x68, frame.Destination, frame.Source, frame.Control)
		wire = append(wire, frame.Payload...)
		wire = append(wire, ppiChecksum(wire[4:]), 0x16)
		return wire, nil
	default:
		return nil, errors.New("unknown PPI frame kind")
	}
}

// DecodePPIFrame validates exactly one frame and copies its payload.
func DecodePPIFrame(wire []byte) (PPIFrame, error) {
	if len(wire) == 0 {
		return PPIFrame{}, errors.New("empty PPI frame")
	}
	var frame PPIFrame
	switch PPIFrameKind(wire[0]) {
	case PPIAck:
		if len(wire) != 1 {
			return frame, errors.New("invalid PPI acknowledgement length")
		}
		return PPIFrame{Kind: PPIAck}, nil
	case PPIFixed:
		if len(wire) != 6 || wire[5] != 0x16 || ppiChecksum(wire[1:4]) != wire[4] {
			return frame, errors.New("invalid PPI fixed frame")
		}
		frame = PPIFrame{Kind: PPIFixed, Destination: wire[1], Source: wire[2], Control: wire[3]}
	case PPIVariable:
		if len(wire) < 9 || wire[1] < 3 || wire[1] != wire[2] || wire[3] != 0x68 || len(wire) != int(wire[1])+6 {
			return frame, errors.New("invalid PPI variable frame length or header")
		}
		last := len(wire) - 1
		if wire[last] != 0x16 || ppiChecksum(wire[4:last-1]) != wire[last-1] {
			return frame, errors.New("invalid PPI variable frame checksum or terminator")
		}
		frame = PPIFrame{Kind: PPIVariable, Destination: wire[4], Source: wire[5], Control: wire[6], Payload: append([]byte{}, wire[7:last-1]...)}
	default:
		return frame, errors.New("unknown PPI frame start")
	}
	if frame.Destination > 126 || frame.Source > 126 {
		return PPIFrame{}, errors.New("PPI address must be 0..126")
	}
	return frame, nil
}

func ppiChecksum(data []byte) byte {
	var sum byte
	for _, value := range data {
		sum += value
	}
	return sum
}
