package net

import (
	"context"
	"errors"
	"fmt"
	gonet "net"
	"sync"
	"syscall"

	"github.com/I-Missha/gonet_uring/ubalancer"
	"github.com/godzie44/go-uring/uring"
)

var (
	ErrTimeout  = errors.New("connection timeout")
	ErrCanceled = errors.New("connection canceled")
)

type UringDialer struct {
	balancer *ubalancer.UBalancer
}

var balancer *ubalancer.UBalancer

func NewUringDialer() *UringDialer {
	if balancer == nil {
		balancer = ubalancer.NewUBalancer(8, 256)
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

	socketChan := make(chan int, 1)
	socketOp := uring.Socket(syscall.AF_INET6, syscall.SOCK_STREAM, 0)

	err = d.balancer.PushOperation(socketOp, func(result int32, err error) {
		if err != nil || result < 0 {
			socketChan <- -1
		} else {
			socketChan <- int(result)
		}
	})

	if err != nil {
		return nil, fmt.Errorf("failed to push socket operation: %w", err)
	}

	fd := <-socketChan
	if fd < 0 {
		return nil, fmt.Errorf("failed to create socket")
	}

	resultChan := make(chan error, 1)

	connectOp := uring.Connect(uintptr(fd), addr)

	err = d.balancer.PushOperation(connectOp, func(result int32, err error) {
		if err != nil {
			resultChan <- err
		} else if result < 0 {
			resultChan <- syscall.Errno(-result)
		} else {
			resultChan <- nil
		}
	})

	if err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("failed to push connect operation: %w", err)
	}

	select {
	case err := <-resultChan:
		if err != nil {
			syscall.Close(fd)
			return nil, fmt.Errorf("connect failed: %w", err)
		}

		return &Conn{fd: fd, mu: sync.Mutex{}, balancer: d.balancer}, nil
	case <-ctx.Done():
		syscall.Close(fd)
		return nil, ErrCanceled
	}
}

