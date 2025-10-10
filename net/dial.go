package net

import (
	"context"
	gonet "net"

	"github.com/godzie44/go-uring/uring"
)

func (d *Dialer) DialContext(ctx context.Context, network, address string) (gonet.Conn, error) {

	return sqe.Fd(), err
}
