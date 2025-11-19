package net

import (
	"context"
	gonet "net"
	"sync"
	"syscall"
	"time"

	"github.com/I-Missha/gonet_uring/ubalancer"
	"github.com/godzie44/go-uring/uring"
)

type Conn struct {
	fd       int
	mu       sync.Mutex
	balancer *ubalancer.UBalancer
	wCount   int
	rCount   int

	// TODO: for the future me :)
	ReadTimout  time.Duration
	WriteTimout time.Duration
	ctx         context.Context
}

func (c *Conn) Read(b []byte) (n int, err error) {
	readOp := uring.Read(uintptr(c.fd), b, 0)
	resultChan := make(chan struct {
		bytes int32
		err   error
	}, 1)

	err = c.balancer.PushOperation(readOp, func(result int32, err error) {
		resultChan <- struct {
			bytes int32
			err   error
		}{result, err}
	})

	if err != nil {
		return 0, err
	}

	var result struct {
		bytes int32
		err   error
	}
	result = <-resultChan
	if result.err != nil {
		return 0, result.err
	}

	c.rCount += int(result.bytes)
	return int(result.bytes), nil
}

func (c *Conn) Write(b []byte) (n int, err error) {
	writeOp := uring.Write(uintptr(c.fd), b, 0)
	resultChan := make(chan struct {
		bytes int32
		err   error
	}, 1)

	err = c.balancer.PushOperation(writeOp, func(result int32, err error) {
		resultChan <- struct {
			bytes int32
			err   error
		}{result, err}
	})

	if err != nil {
		return 0, err
	}

	var result struct {
		bytes int32
		err   error
	}
	result = <-resultChan
	if result.err != nil {
		return 0, result.err
	}

	c.wCount += int(result.bytes)
	return int(result.bytes), nil
}

func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fd >= 0 { // zero ??
		err := syscall.Close(c.fd)
		c.fd = -1
		return err
	}
	return nil
}

func (c *Conn) LocalAddr() gonet.Addr {
	return nil
}

func (c *Conn) RemoteAddr() gonet.Addr {
	return nil
}

func (c *Conn) SetDeadline(t time.Time) error {
	return nil
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	return nil
}
