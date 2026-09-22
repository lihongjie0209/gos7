package gos7

import (
	"bytes"
	"testing"
)

func TestMPI2ConnectPayloads(t *testing.T) {
	t.Parallel()
	offer, err := EncodeMPI2PLCConnectOffer(2, 3)
	wantOffer := []byte{0, 0x0d, 0, 3, 0xe0, 4, 0, 0x80, 0, 2, 1, 6, 1, 0, 0, 1, 2, 2, 1, 0}
	if err != nil || !bytes.Equal(offer, wantOffer) {
		t.Fatalf("offer=%x err=%v want=%x", offer, err, wantOffer)
	}
	accept := []byte{0, 0x0c, 0x44, 0x55, 0xd0, 4, 0, 0x80, 1, 6, 0, 2, 0, 1, 2, 2, 1, 0, 1, 0}
	peer, err := DecodeMPI2PLCConnectAccept(accept, 2)
	if err != nil || peer != 0x55 {
		t.Fatalf("peer=%x err=%v", peer, err)
	}
	confirm, err := EncodeMPI2PLCConnectConfirm(peer, 3)
	if err != nil || !bytes.Equal(confirm, []byte{0, 0x0c, 0x55, 3, 5, 1}) {
		t.Fatalf("confirm=%x err=%v", confirm, err)
	}
	if err := DecodeMPI2PLCConnectConfirmed([]byte{0, 0x0c, 0xaa, 0xbb, 5, 1}); err != nil {
		t.Fatal(err)
	}
}

func TestMPI2ConnectPayloadsRejectMalformed(t *testing.T) {
	t.Parallel()
	if payload, err := EncodeMPI2PLCConnectOffer(127, 3); err == nil || payload != nil {
		t.Fatalf("invalid station: %x %v", payload, err)
	}
	if payload, err := EncodeMPI2PLCConnectOffer(2, 0); err == nil || payload != nil {
		t.Fatalf("zero connection: %x %v", payload, err)
	}
	if payload, err := EncodeMPI2PLCConnectConfirm(1, 0); err == nil || payload != nil {
		t.Fatalf("zero local connection: %x %v", payload, err)
	}
	accept := []byte{0, 0x0c, 0x44, 0x55, 0xd0, 4, 0, 0x80, 1, 6, 0, 2, 0, 1, 2, 2, 1, 0, 1, 0}
	for _, size := range []int{0, 19, 21} {
		response := bytes.Clone(accept)
		if size == 21 {
			response = append(response, 0)
		} else {
			response = response[:size]
		}
		if _, err := DecodeMPI2PLCConnectAccept(response, 2); err == nil {
			t.Fatalf("accepted length %d", size)
		}
	}
	if _, err := DecodeMPI2PLCConnectAccept(accept, 127); err == nil {
		t.Fatal("accepted invalid target station")
	}
	if _, err := DecodeMPI2PLCConnectAccept(accept, 3); err == nil {
		t.Fatal("accepted wrong station")
	}
	for index := range accept {
		if index == 2 || index == 3 {
			continue
		}
		response := bytes.Clone(accept)
		response[index] ^= 0xff
		if _, err := DecodeMPI2PLCConnectAccept(response, 2); err == nil {
			t.Fatalf("accepted changed fixed byte %d", index)
		}
	}
	confirmed := []byte{0, 0x0c, 0xaa, 0xbb, 5, 1}
	for _, response := range [][]byte{nil, confirmed[:5], append(bytes.Clone(confirmed), 0)} {
		if err := DecodeMPI2PLCConnectConfirmed(response); err == nil {
			t.Fatalf("accepted length %d", len(response))
		}
	}
	for _, index := range []int{0, 1, 4, 5} {
		response := bytes.Clone(confirmed)
		response[index] ^= 0xff
		if err := DecodeMPI2PLCConnectConfirmed(response); err == nil {
			t.Fatalf("accepted changed fixed byte %d", index)
		}
	}
}
