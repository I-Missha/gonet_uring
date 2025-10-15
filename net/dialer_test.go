package net

import (
	"context"
	"testing"
	"net"
	"net/http"
)

// :D
func TestDialer(t *testing.T) {
	stopServer := startServer(t, ":8080")
	defer stopServer()
	d := NewUringDialer()
	conn, err := d.DialContext(context.Background(), "tcp", "127.0.0.1:8080")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
}

func startServer(t *testing.T, addr string) func() {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		_ = http.Serve(l, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		}))
	}()

	return func() {
		l.Close()
	}
}

