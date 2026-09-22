package gos7

// Copyright 2018 Trung Hieu Le. All rights reserved.
// This software may be modified and distributed under the terms
// of the BSD license. See the LICENSE file for details.
import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"
)

func TestNewTCPClientHandlerWithTSAP(t *testing.T) {
	handler := NewTCPClientHandlerWithTSAP("192.0.2.10:1102", 0x1201, 0x3402)
	if handler.Address != "192.0.2.10:1102" {
		t.Fatalf("address=%q", handler.Address)
	}
	if handler.localTSAPHigh != 0x12 || handler.localTSAPLow != 0x01 || handler.remoteTSAPHigh != 0x34 || handler.remoteTSAPLow != 0x02 {
		t.Fatalf("local=%02x%02x remote=%02x%02x", handler.localTSAPHigh, handler.localTSAPLow, handler.remoteTSAPHigh, handler.remoteTSAPLow)
	}
}

func TestTCPTransporter(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		_, err = io.Copy(conn, conn)
		if err != nil {
			t.Error(err)
			return
		}
	}()
	client := &tcpTransporter{
		Address:     ln.Addr().String(),
		Timeout:     200 * time.Second,
		IdleTimeout: 100 * time.Millisecond,
	}
	req := []byte{0, 1, 0, 17, 0, 2, 1, 2, 0, 1, 0, 17, 0, 2, 1, 2, 2} //lengh 17, > MinPduSize

	client.tcpConnect(context.Background()) //assume tcp connect to test locally
	rsp, err := client.Send(req)
	if err != nil {
		t.Fatal(err)
	}
	//lth: just compare 7 first byte
	if !bytes.Equal(req, rsp) {
		t.Fatalf("unexpected response: %x", rsp)
	}
	time.Sleep(150 * time.Millisecond)
	client.mu.Lock()
	connection := client.conn
	client.mu.Unlock()
	if connection != nil {
		t.Fatalf("connection is not closed: %+v", connection)
	}
}
