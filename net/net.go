package net

import (
	"errors"
	gonet "net"
	"sync"
	"syscall"
	"time"

	"github.com/godzie44/go-uring/uring"
)

var ErrTimeout = errors.New("operation timed out")

// UringConn реализует net.Conn через io_uring
type UringConn struct {
	fd         int
	ring       *uring.Ring
	localAddr  gonet.Addr
	remoteAddr gonet.Addr
	readLock   sync.Mutex
	writeLock  sync.Mutex
}

// New создает новое соединение поверх io_uring
func New(fd int, localAddr, remoteAddr gonet.Addr) (*UringConn, error) {
	ring, err := uring.New(1)
	if err != nil {
		return nil, err
	}

	return &UringConn{
		fd:         fd,
		ring:       ring,
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
	}, nil
}

func (c *UringConn) Read(b []byte) (int, error) {
	c.readLock.Lock()
	defer c.readLock.Unlock()

	return c.read(b, nil)
}

func (c *UringConn) Write(b []byte) (int, error) {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	return c.write(b, nil)
}

func (c *UringConn) Close() error {
	if c.ring != nil {
		c.ring.Close()
		c.ring = nil
	}
	if c.fd != 0 {
		err := syscall.Close(c.fd)
		c.fd = 0
		return err
	}
	return nil
}

func (c *UringConn) LocalAddr() gonet.Addr {
	return c.localAddr
}

func (c *UringConn) RemoteAddr() gonet.Addr {
	return c.remoteAddr
}

func (c *UringConn) SetDeadline(t time.Time) error {
	return c.SetReadDeadline(t)
}

func (c *UringConn) SetReadDeadline(t time.Time) error {
	// Реализация через POLL в отдельном методе
	return nil
}

func (c *UringConn) SetWriteDeadline(t time.Time) error {
	// Аналогично SetReadDeadline
	return nil
}

// Внутренние методы с поддержкой таймаутов
func (c *UringConn) read(b []byte, deadline *time.Time) (int, error) {
	sqe := c.ring.GetSQEntry()
	if sqe == nil {
		return 0, errors.New("failed to get SQE")
	}

	uring.Read(sqe, c.fd, b, 0)

	if _, err := c.ring.Submit(1, nil); err != nil {
		return 0, err
	}

	if deadline != nil {
		if err := c.setPollTimeout(deadline); err != nil {
			return 0, err
		}
	}

	cqe, err := c.ring.WaitCQEvents(1)
	if err != nil {
		return 0, err
	}
	defer c.ring.CQESeen(cqe)

	if cqe.Res < 0 {
		return 0, syscall.Errno(-cqe.Res)
	}

	return int(cqe.Res), nil
}

func (c *UringConn) write(b []byte, deadline *time.Time) (int, error) {
	sqe := c.ring.GetSQEntry()
	if sqe == nil {
		return 0, errors.New("failed to get SQE")
	}

	uring.Write(sqe, c.fd, b, 0)

	if _, err := c.ring.Submit(1, nil); err != nil {
		return 0, err
	}

	if deadline != nil {
		if err := c.setPollTimeout(deadline); err != nil {
			return 0, err
		}
	}

	cqe, err := c.ring.WaitCQEvents(1)
	if err != nil {
		return 0, err
	}
	defer c.ring.CQESeen(cqe)

	if cqe.Res < 0 {
		return 0, syscall.Errno(-cqe.Res)
	}

	return int(cqe.Res), nil
}

func (c *UringConn) setPollTimeout(deadline *time.Time) error {
	timeoutSpec := uring.Timespec{
		Sec:  deadline.Unix(),
		Nsec: int64(deadline.Nanosecond()),
	}

	sqe := c.ring.GetSQEntry()
	if sqe == nil {
		return errors.New("failed to get SQE for timeout")
	}

	uring.LinkTimeout(sqe, &timeoutSpec, 0)
	_, err := c.ring.Submit(1, nil)
	return err
}
