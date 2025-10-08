package net

import (
	"context"
	"syscall"
	gonet "net"

	"github.com/godzie44/go-uring/uring"
)

type Dialer struct {
	ring uring.Ring

}

func (d *Dialer) DialContext(ctx context.Context, network, address string) (gonet.Conn, error) {
	sockFd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}

	/*
	if err = setDefaultListenerSockopts(sockFd); err != nil {
		return nil, err
	}
	*/

	addr, err := gonet.ResolveTCPAddr(network, address)
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	connOp := uring.Connect(uintptr(sockFd), addr)
	sqe := uring.SQEntry{}
	connOp.PrepSQE(&sqe)
	d.ring.QueueSQE(connOp, 0, 0)

	return sqe.Fd(), err
}
