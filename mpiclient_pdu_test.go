package gos7

import (
	"bytes"
	"testing"
)

func TestMPI2PDUCodec(t *testing.T) {
	t.Parallel()
	request := []byte{0x32, 1, 0, 0, 0, 1, 0, 2, 0, 0, 0xf0, 0}
	wantPDU := bytes.Clone(request)
	envelope, err := EncodeMPI2PDUEnvelope(0x55, 3, 7, request)
	if err != nil || !bytes.Equal(envelope, append([]byte{0, 0x0c, 0x55, 3, 0xf1, 7}, request...)) {
		t.Fatalf("envelope=%x err=%v", envelope, err)
	}
	request[0] = 0
	if envelope[6] != 0x32 {
		t.Fatal("encoded envelope aliases request")
	}
	number, decoded, err := DecodeMPI2PDUEnvelope(envelope, 0x55, 3)
	if err != nil || number != 7 || !bytes.Equal(decoded, wantPDU) {
		t.Fatalf("number=%d decoded=%x err=%v", number, decoded, err)
	}
	envelope[6] = 0
	if decoded[0] != 0x32 {
		t.Fatal("decoded PDU aliases envelope")
	}
	response := []byte{0x32, 3, 0, 0, 0, 1, 0, 8, 0, 0, 0, 0, 0xf0, 0, 0, 1, 0, 1, 0, 0xc0}
	envelope, err = EncodeMPI2PDUEnvelope(0x55, 3, 8, response)
	if err != nil {
		t.Fatal(err)
	}
	number, decoded, err = DecodeMPI2PDUEnvelope(envelope, 0x55, 3)
	if err != nil || number != 8 || !bytes.Equal(decoded, response) {
		t.Fatalf("response number=%d decoded=%x err=%v", number, decoded, err)
	}
	ack, err := EncodeMPI2MessageAck(0x55, 3, 8)
	if err != nil || !bytes.Equal(ack, []byte{0, 0x0c, 0x55, 3, 0xb0, 1, 8}) {
		t.Fatalf("ack=%x err=%v", ack, err)
	}
	if err := DecodeMPI2MessageAck(ack, 0x55, 3, 8); err != nil {
		t.Fatal(err)
	}
}

func TestMPI2PDUCodecRejectsMalformed(t *testing.T) {
	t.Parallel()
	request := []byte{0x32, 1, 0, 0, 0, 1, 0, 2, 0, 0, 0xf0, 0}
	valid, _ := EncodeMPI2PDUEnvelope(0x55, 3, 7, request)
	tests := []struct {
		name    string
		payload []byte
		peer    byte
		local   byte
	}{
		{name: "short", payload: valid[:15], peer: 0x55, local: 3},
		{name: "long", payload: append(bytes.Clone(valid), 0), peer: 0x55, local: 3},
		{name: "peer", payload: valid, peer: 0x56, local: 3},
		{name: "local", payload: valid, peer: 0x55, local: 4},
		{name: "zero local", payload: valid, peer: 0x55, local: 0},
		{name: "marker", payload: func() []byte { b := bytes.Clone(valid); b[4] = 0; return b }(), peer: 0x55, local: 3},
		{name: "protocol", payload: func() []byte { b := bytes.Clone(valid); b[6] = 0; return b }(), peer: 0x55, local: 3},
		{name: "length", payload: func() []byte { b := bytes.Clone(valid); b[13] = 7; return b }(), peer: 0x55, local: 3},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, got, err := DecodeMPI2PDUEnvelope(tt.payload, tt.peer, tt.local); err == nil || got != nil {
				t.Fatalf("accepted malformed PDU: %x %v", got, err)
			}
		})
	}
	for _, pdu := range [][]byte{nil, {0x32}, bytes.Repeat([]byte{0x32}, 2048)} {
		if _, err := EncodeMPI2PDUEnvelope(0x55, 3, 7, pdu); err == nil {
			t.Fatalf("accepted invalid PDU of %d bytes", len(pdu))
		}
	}
	if _, err := EncodeMPI2PDUEnvelope(0x55, 0, 7, request); err == nil {
		t.Fatal("accepted zero local connection")
	}
	ack, _ := EncodeMPI2MessageAck(0x55, 3, 8)
	for _, tt := range []struct {
		name                string
		ack                 []byte
		peer, local, number byte
	}{
		{name: "short", ack: ack[:6], peer: 0x55, local: 3, number: 8},
		{name: "long", ack: append(bytes.Clone(ack), 0), peer: 0x55, local: 3, number: 8},
		{name: "peer", ack: ack, peer: 0x56, local: 3, number: 8},
		{name: "local", ack: ack, peer: 0x55, local: 4, number: 8},
		{name: "number", ack: ack, peer: 0x55, local: 3, number: 9},
		{name: "marker", ack: func() []byte { b := bytes.Clone(ack); b[4] = 0; return b }(), peer: 0x55, local: 3, number: 8},
	} {
		tt := tt
		t.Run("ack "+tt.name, func(t *testing.T) {
			t.Parallel()
			if err := DecodeMPI2MessageAck(tt.ack, tt.peer, tt.local, tt.number); err == nil {
				t.Fatal("accepted malformed acknowledgement")
			}
		})
	}
}
