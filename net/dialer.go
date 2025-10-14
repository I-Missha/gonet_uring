package net

import (
	"context"
	"errors"
	"fmt"
	gonet "net"
	"syscall"
	"sync"
	"time"

	"github.com/I-Missha/gonet_uring/ubalancer"
	"github.com/godzie44/go-uring/uring"
)

var (
	ErrTimeout = errors.New("connection timeout")
	ErrCanceled = errors.New("connection canceled")
)

type UringDialer struct {
	balancer *ubalancer.UBalancer
}

func NewUringDialer() *UringDialer {
	balancer := ubalancer.NewUBalancer(4, 16)
	balancer.Run()
	return &UringDialer{
		balancer: balancer,
	}
}

func (d *UringDialer) DialContext(ctx context.Context, network, address string) (gonet.Conn, error) {
	// Парсим адрес
	addr, err := gonet.ResolveTCPAddr(network, address)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve address: %w", err)
	}

	socketChan := make(chan int, 1)
	socketOp := uring.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)

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

	// Конвертируем адрес в syscall.Sockaddr
	sockaddr := &syscall.SockaddrInet4{Port: addr.Port}
	copy(sockaddr.Addr[:], addr.IP.To4())

	// Канал для результата подключения
	resultChan := make(chan error, 1)

	// Создаем операцию connect
	connectOp := uring.Connect(uintptr(fd), addr)

	// Отправляем операцию в балансер
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

	// Ждем результат или контекст
	select {
	case err := <-resultChan:
		if err != nil {
			syscall.Close(fd)
			return nil, fmt.Errorf("connect failed: %w", err)
		}
		// Возвращаем временную заглушку вместо полной реализации net.Conn
		return &stubConn{fd: fd}, nil
	case <-ctx.Done():
		syscall.Close(fd)
		return nil, ErrCanceled
	}
}

// Временная заглушка для net.Conn
type stubConn struct {
	fd int
	mu sync.Mutex
}

func (c *stubConn) Read(b []byte) (n int, err error) {
	return 0, errors.New("not implemented")
}

func (c *stubConn) Write(b []byte) (n int, err error) {
	return 0, errors.New("not implemented")
}

func (c *stubConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fd >= 0 {
		err := syscall.Close(c.fd)
		c.fd = -1
		return err
	}
	return nil
}

func (c *stubConn) LocalAddr() gonet.Addr {
	return nil
}

func (c *stubConn) RemoteAddr() gonet.Addr {
	return nil
}

func (c *stubConn) SetDeadline(t time.Time) error {
	return errors.New("not implemented")
}

func (c *stubConn) SetReadDeadline(t time.Time) error {
	return errors.New("not implemented")
}

func (c *stubConn) SetWriteDeadline(t time.Time) error {
	return errors.New("not implemented")
}

