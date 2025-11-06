package net

import (
	gonet "net"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

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

	var readMu sync.Mutex
	readCond := sync.NewCond(&readMu)
	var readDone bool = false
	var readResult int32
	var readError error

	callb := func(result int32, err error) {
		readMu.Lock()
		defer readMu.Unlock()
		readResult = result
		readError = err
		readDone = true
		readCond.Signal()
	}
	cb := uint64(uintptr(unsafe.Pointer(&callb)))

	err = c.balancer.PushOperation(readOp, cb)

	if err != nil {
		return 0, err
	}

	readMu.Lock()
	for !readDone {
		readCond.Wait()
	}
	bytes := readResult
	err = readError
	readMu.Unlock()

	runtime.KeepAlive(callb)
	if err != nil {
		return 0, err
	}
	c.rCount += int(bytes)

	return int(bytes), nil
}

func (c *Conn) Write(b []byte) (n int, err error) {
	writeOp := uring.Write(uintptr(c.fd), b, 0)

	var writeMu sync.Mutex
	writeCond := sync.NewCond(&writeMu)
	var writeDone bool = false
	var writeResult int32
	var writeError error

	callb := func(result int32, err error) {
		writeMu.Lock()
		defer writeMu.Unlock()
		writeResult = result
		writeError = err
		writeDone = true
		writeCond.Signal()
	}
	cb := uint64(uintptr(unsafe.Pointer(&callb)))

	err = c.balancer.PushOperation(writeOp, cb)

	if err != nil {
		return 0, err
	}

	writeMu.Lock()
	for !writeDone {
		writeCond.Wait()
	}
	bytes := writeResult
	err = writeError
	writeMu.Unlock()

	runtime.KeepAlive(callb)
	if err != nil {
		return 0, err
	}
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
