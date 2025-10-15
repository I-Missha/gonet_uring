package net

import (
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
}

func (c *Conn) Read(b []byte) (n int, err error) {
	readOp := uring.Read(uintptr(c.fd), b, 0)
	bytesNum := make(chan int32)
	err = c.balancer.PushOperation(readOp, func(result int32, err error) {
		if err != nil {
			return
		}
		bytesNum <- result

	})

	if err != nil {
		return 0, err
	}

	bytes := <-bytesNum
	c.rCount += int(bytes)

	return int(bytes), nil
}

func (c *Conn) Write(b []byte) (n int, err error) {
	writeOp := uring.Write(uintptr(c.fd), b, 0)
	bytesNum := make(chan int32)
	err = c.balancer.PushOperation(writeOp, func(result int32, err error) {
		if err != nil {
			return
		}
		bytesNum <- result

	})

	if err != nil {
		return 0, err
	}

	bytes := <-bytesNum
	c.wCount += int(bytes)

	return int(bytes), nil
}

func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fd >= 0 {
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
