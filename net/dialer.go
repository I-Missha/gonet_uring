package net

import (
	"context"
	"errors"
	"fmt"
	gonet "net"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/I-Missha/gonet_uring/ubalancer"
	"github.com/godzie44/go-uring/uring"
)

var (
	ErrTimeout  = errors.New("connection timeout")
	ErrCanceled = errors.New("connection canceled")
)

type UringDialer struct {
	balancer *ubalancer.UBalancer
	success  int64
}

var balancer *ubalancer.UBalancer

func NewUringDialer() *UringDialer {
	if balancer == nil {
		balancer = ubalancer.NewUBalancer(8, 64)
		balancer.Run()
	}

	return &UringDialer{
		balancer: balancer,
	}
}

func (d *UringDialer) DialContext(ctx context.Context, network, address string) (gonet.Conn, error) {
	addr, err := gonet.ResolveTCPAddr(network, address)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve address: %w", err)
	}

	socketOp := uring.Socket(syscall.AF_INET6, syscall.SOCK_STREAM, 0)

	ch := make(chan int32)
	callb := func(result int32, err error) {
		ch <- result
	}
	cb := uint64(uintptr(unsafe.Pointer(&callb)))

	err = d.balancer.PushOperation(socketOp, cb)

	if err != nil {
		return nil, fmt.Errorf("failed to push socket operation: %w", err)
	}
	fd := int(<-ch)

	if fd < 0 {
		return nil, fmt.Errorf("failed to create socket")
	}
	runtime.KeepAlive(callb)


	connectOp := uring.Connect(uintptr(fd), addr)

	callb = func(result int32, err error) {
		ch <- result
	}

	cb = uint64(uintptr(unsafe.Pointer(&callb)))

	err = d.balancer.PushOperation(connectOp, cb)

	if err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("failed to push connect operation: %w", err)
	}

	select {
	case <-ch:
		runtime.KeepAlive(callb)
		atomic.AddInt64(&d.success, 1)
		return &Conn{fd: fd, mu: sync.Mutex{}, balancer: d.balancer}, nil
	case <-ctx.Done():
		syscall.Close(fd)
		runtime.KeepAlive(callb)
		return nil, ErrCanceled
	}
}

func (d *UringDialer) GetSuccessCount() int {
	return int(d.success)
}
