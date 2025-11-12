package bench

import (
	"context"
	"net"
	"os"
	"runtime"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	mynet "github.com/I-Missha/gonet_uring/net"
	"github.com/valyala/fasthttp"
)

type benchCfg struct {
	metriscUrl string
}

func TestMain(m *testing.M) {

	os.Exit(m.Run())
}

func BenchmarkClientGetEndToEnd4000TCP(b *testing.B) {
	b.Run("io_uring", func(b *testing.B) {
		benchmarkClientGetEndToEndTCPNoKeepAlive(b, 125, func(addr string) (net.Conn, error) {
			dialer := mynet.NewUringDialer()
			return dialer.DialContext(context.TODO(), "tcp", addr)
		})
	})

	time.Sleep(time.Second * 20)

	b.Run("net", func(b *testing.B) {
		benchmarkClientGetEndToEndTCPNoKeepAlive(b, 125, func(addr string) (net.Conn, error) {
			return net.Dial("tcp6", addr)
		})
	})
}

func benchmarkClientGetEndToEndTCPNoKeepAlive(b *testing.B, parallelism int, dial fasthttp.DialFunc) {
	addresses := make([]string, 0, 5)

	addresses = append(addresses,
		"2a02:6b8:c02:901:0:fc7e:0:36c",
		"2a02:6b8:c02:901:0:fc7e:0:3cf",
		"2a02:6b8:c02:901:0:fc7e:0:398",
		"2a02:6b8:c02:901:0:fc7e:0:3df",
		"2a02:6b8:c02:901:0:fc7e:0:345",
		"2a02:6b8:c02:901:0:fc7e:0:261",
		"2a02:6b8:c02:901:0:fc7e:0:1a8")

	// Клиенту теперь не нужен большой пул соединений, но оставим его,
	c := &fasthttp.Client{
		// MaxConnsPerHost больше не играет роли для переиспользования,
		// но все еще ограничивает количество одновременных запросов в полете.
		MaxConnsPerHost:    runtime.GOMAXPROCS(-1) * parallelism,
		Dial:               dial,
		MaxConnWaitTimeout: time.Second * 5,
	}

	requestURI := "/hello"

	amountPorts := 10
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
	b.SetParallelism(parallelism * runtime.GOMAXPROCS(-1))

	b.ResetTimer()

	amount := uint32(len(urls))
	var counter uint32
	b.RunParallel(func(pb *testing.PB) {
		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req) // Убедимся, что объект вернется в пул при выходе из горутины

		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(resp) // То же самое для ответа

		// req.SetRequestURI(urls[int(counter%uint32(amount))])

		req.Header.Set("Connection", "close")

		for pb.Next() {
			// Выполняем запрос с помощью c.Do
			req.SetRequestURI(urls[int(atomic.AddUint32(&counter, 1)%uint32(amount))])
			if err := c.Do(req, resp); err != nil {
				b.Fatalf("unexpected error: %v", err)
			}

			statusCode := resp.StatusCode()
			body := resp.Body()

			if statusCode != fasthttp.StatusOK {
				b.Fatalf("unexpected status code: %d. Expecting %d", statusCode, fasthttp.StatusOK)
			}
			if string(body) != requestURI {
				b.Fatalf("unexpected response %q. Expecting %q", body, requestURI)
			}

			resp.ResetBody()
		}
	})

}
