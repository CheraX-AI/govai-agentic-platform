package postgres

import (
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

// Drop one real broker confirm after RabbitMQ has accepted the test message.

func dropConfirmProxy(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal("confirm-loss proxy unavailable")
	}
	finished := make(chan struct{})
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
			t.Error("confirm-loss proxy did not stop")
		}
	})
	go func() {
		defer close(finished)
		client, err := listener.Accept()
		if err != nil {
			return
		}
		defer client.Close()
		broker, err := net.DialTimeout("tcp", "127.0.0.1:5672", time.Second)
		if err != nil {
			return
		}
		defer broker.Close()
		_ = client.SetDeadline(time.Now().Add(5 * time.Second))
		_ = broker.SetDeadline(time.Now().Add(5 * time.Second))
		go func() { _, _ = io.Copy(broker, client) }()
		for {
			header := make([]byte, 7)
			if _, err := io.ReadFull(broker, header); err != nil {
				return
			}
			size := binary.BigEndian.Uint32(header[3:7])
			if size > 1<<20 {
				return
			}
			body := make([]byte, int(size)+1)
			if _, err := io.ReadFull(broker, body); err != nil {
				return
			}
			if header[0] == 1 && size >= 4 &&
				binary.BigEndian.Uint16(body[:2]) == 60 &&
				binary.BigEndian.Uint16(body[2:4]) == 80 {
				return
			}
			if _, err := client.Write(append(header, body...)); err != nil {
				return
			}
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port
}
