package test

import (
	"context"
	"net"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

// 	mynet "github.com/I-Missha/gonet_uring/net"
	"github.com/valyala/fasthttp"
)

type benchCfg struct {
	metriscUrl string
}

func TestMain(m *testing.M) {

	os.Exit(m.Run())
}

func TestFunction(b *testing.T) {
	parallelism := 5000
	/*
		b.Run("io_uring", func(b *testing.T) {
			benchmarkClientGetEndToEndTCPNoKeepAlive(b, parallelism, func(addr string, timeout time.Duration) (net.Conn, error) {
				dialer := mynet.NewUringDialer()
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()
				return dialer.DialContext(ctx, "tcp", addr)
			})
		})

		time.Sleep(time.Second * 60)

	*/
	b.Run("net", func(b *testing.T) {
		benchmarkClientGetEndToEndTCPNoKeepAlive(b, parallelism, func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("tcp6", addr, timeout)
		})
	})
}

var sink any

func cpuWork(iterations int) {
	var result int
	for i := 0; i < iterations; i++ {
		result += i * i / (i + 1)
	}
	sink = result
}
func benchmarkClientGetEndToEndTCPNoKeepAlive(b *testing.T, parallelism int, dial fasthttp.DialFuncWithTimeout) {

	addresses := make([]string, 0, 5)

	addresses = append(addresses,
		"2a02:6b8:c02:901:0:fc7e:0:36c",
		"2a02:6b8:c02:901:0:fc7e:0:3cf",
		"2a02:6b8:c02:901:0:fc7e:0:398",
		"2a02:6b8:c02:901:0:fc7e:0:3df",
		"2a02:6b8:c02:901:0:fc7e:0:345",
		"2a02:6b8:c02:901:0:fc7e:0:171",
		"2a02:6b8:c02:901:0:fc7e:0:38c",
		"2a02:6b8:c02:901:0:fc7e:0:369",
		"2a02:6b8:c02:901:0:fc7e:0:c2",
		"2a02:6b8:c02:901:0:fc7e:0:2f4",
		"2a02:6b8:c02:901:0:fc7e:0:23",
		"2a02:6b8:c02:901:0:fc7e:0:1c2",
		"2a02:6b8:c02:901:0:fc7e:0:14a",
		"2a02:6b8:c02:901:0:fc7e:0:58",
	)

	c := &fasthttp.Client{
		MaxConnsPerHost:           runtime.GOMAXPROCS(-1) * parallelism,
		DialTimeout:               dial,
		MaxIdemponentCallAttempts: 1,
	}

	requestURI := "/h"
	amountPorts := 1
	basePort := 8543
	urls := make([]string, amountPorts*len(addresses))
	for i := 0; i < amountPorts; i++ {
		port := basePort + i
		for j := 0; j < len(addresses); j++ {
			addr := addresses[j]
			resAddr := net.JoinHostPort(addr, strconv.Itoa(port))

			urls[i*len(addresses)+j] = "http://" + resAddr + requestURI
		}
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second*180))
	defer cancel()
	var wg sync.WaitGroup

	for range runtime.GOMAXPROCS(-1) * 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					cpuWork(10)
				}
			}
		}()

	}

	header := &fasthttp.RequestHeader{}
	header.SetConnectionClose()
	header.SetProtocol("HTTP/1.1")

	var counter uint64
	var timeoutCounter uint64
	for range parallelism {

		wg.Add(1)
		go func() {
			defer wg.Done()
			req := fasthttp.AcquireRequest()
			defer fasthttp.ReleaseRequest(req) // Убедимся, что объект вернется в пул при выходе из горутины
			req.SetTimeout(time.Second * 2)

			resp := fasthttp.AcquireResponse()
			defer fasthttp.ReleaseResponse(resp) // То же самое для ответа

			// req.SetRequestURI(urls[int(counter%uint32(amount))])
			header.CopyTo(&req.Header)

			amount := uint64(len(urls))
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Выполняем запрос с помощью c.Do

					req.SetRequestURI(urls[int(atomic.AddUint64(&counter, 1)%amount)])
					counter++
					if err := c.Do(req, resp); err != nil {
						b.Errorf("unexpected error: %v", err)
						atomic.AddUint64(&timeoutCounter, 1)
						continue
					}

					statusCode := resp.StatusCode()

					if statusCode != fasthttp.StatusOK {
						b.Errorf("unexpected status code: %d. Expecting %d", statusCode, fasthttp.StatusOK)
						return
					}

					resp.ResetBody()
				}
			}
		}()
	}

	wg.Wait()
	b.Logf("all Do opertaions: %d", counter)
	b.Logf("timeouts: %d", timeoutCounter)
}
