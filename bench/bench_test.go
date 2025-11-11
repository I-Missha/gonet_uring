package bench

import (
	"context"
	"net"
	"os"
	"runtime"
	"testing"
	"time"

	mynet "github.com/I-Missha/gonet_uring/net"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/valyala/fasthttp"
)

var (
	tcpAccess = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tcp_access",
		Help: "Number of TCP accesses",
	},
		[]string{"method"},
	)
)

type benchCfg struct {
	metriscUrl string
}

var customRegistry = prometheus.NewRegistry()

func TestMain(m *testing.M) {

	customRegistry.MustRegister(tcpAccess)

	os.Exit(m.Run())
}

func BenchmarkClientGetEndToEnd1000TCP(b *testing.B) {
	b.Run("io_uring", func(b *testing.B) {
		benchmarkClientGetEndToEndTCPNoKeepAlive(b, 1000, func(addr string) (net.Conn, error) {
			dialer := mynet.NewUringDialer()
			return dialer.DialContext(context.TODO(), "tcp", addr)
		})
	})

	time.Sleep(time.Second * 20)

	b.Run("net", func(b *testing.B) {
		benchmarkClientGetEndToEndTCPNoKeepAlive(b, 1000, func(addr string) (net.Conn, error) {
			return net.Dial("tcp6", addr)
		})
	})
}

func benchmarkClientGetEndToEndTCPNoKeepAlive(b *testing.B, parallelism int, dial fasthttp.DialFunc) {
	addr := "[2a02:6b8:c02:901:0:fc7e:0:36c]:8543" // TODO: set addr to config.yaml

	// Клиенту теперь не нужен большой пул соединений, но оставим его,
	c := &fasthttp.Client{
		// MaxConnsPerHost больше не играет роли для переиспользования,
		// но все еще ограничивает количество одновременных запросов в полете.
		MaxConnsPerHost: runtime.GOMAXPROCS(-1) * parallelism,
		Dial:            dial,
	}

	requestURI := "/hello"
	url := "http://" + addr + requestURI
	b.SetParallelism(parallelism)

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req) // Убедимся, что объект вернется в пул при выходе из горутины

		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(resp) // То же самое для ответа

		req.SetRequestURI(url)

		req.Header.Set("Connection", "close")

		for pb.Next() {
			// Выполняем запрос с помощью c.Do
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
