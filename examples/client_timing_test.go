package fasthttp

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/valyala/fasthttp/fasthttputil"
)

type fakeClientConn struct {
	net.Conn

	ch chan struct{}
	s  []byte
	n  int
}

func (c *fakeClientConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (c *fakeClientConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *fakeClientConn) Write(b []byte) (int, error) {
	c.ch <- struct{}{}
	return len(b), nil
}

func (c *fakeClientConn) Read(b []byte) (int, error) {
	if c.n == 0 {
		// wait for request :)
		<-c.ch
	}
	n := 0
	for len(b) > 0 {
		if c.n == len(c.s) {
			c.n = 0
			return n, nil
		}
		n = copy(b, c.s[c.n:])
		c.n += n
		b = b[n:]
	}
	return n, nil
}

func (c *fakeClientConn) Close() error {
	releaseFakeServerConn(c)
	return nil
}

func (c *fakeClientConn) LocalAddr() net.Addr {
	return &net.TCPAddr{
		IP:   []byte{1, 2, 3, 4},
		Port: 8765,
	}
}

func (c *fakeClientConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{
		IP:   []byte{1, 2, 3, 4},
		Port: 8765,
	}
}

func releaseFakeServerConn(c *fakeClientConn) {
	c.n = 0
	fakeClientConnPool.Put(c)
}

func acquireFakeServerConn(s []byte) *fakeClientConn {
	v := fakeClientConnPool.Get()
	if v == nil {
		c := &fakeClientConn{
			s:  s,
			ch: make(chan struct{}, 1),
		}
		return c
	}
	return v.(*fakeClientConn)
}

var fakeClientConnPool sync.Pool

func BenchmarkClientGetTimeoutFastServer(b *testing.B) {
	body := []byte("123456789099")
	s := []byte(fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(body), body))
	c := &Client{
		Dial: func(addr string) (net.Conn, error) {
			return acquireFakeServerConn(s), nil
		},
	}

	var nn atomic.Uint32
	b.RunParallel(func(pb *testing.PB) {
		url := fmt.Sprintf("http://foobar%d.com/aaa/bbb", nn.Add(1))
		var statusCode int
		var bodyBuf []byte
		var err error
		for pb.Next() {
			statusCode, bodyBuf, err = c.GetTimeout(bodyBuf[:0], url, time.Second)
			if err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
			if statusCode != StatusOK {
				b.Fatalf("unexpected status code: %d", statusCode)
			}
			if !bytes.Equal(bodyBuf, body) {
				b.Fatalf("unexpected response body: %q. Expected %q", bodyBuf, body)
			}
		}
	})
}

func fasthttpEchoHandler(ctx *RequestCtx) {
	ctx.Success("text/plain", ctx.RequestURI())
}

func nethttpEchoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(HeaderContentType, "text/plain")
	w.Write([]byte(r.RequestURI)) //nolint:errcheck
}

/*
func BenchmarkClientGetEndToEnd1TCP(b *testing.B) {
	b.Run("io_uring", func(b *testing.B) {
		benchmarkClientGetEndToEndTCP(b, 1, func(addr string) (net.Conn, error) {
			dialer := mynet.NewUringDialer()
			return dialer.DialContext(context.TODO(), "tcp", addr)
		})
	})

	b.Run("net", func(b *testing.B) {
		benchmarkClientGetEndToEndTCP(b, 1, func(addr string) (net.Conn, error) {
			return net.Dial("tcp", addr)
		})
	})

}

func BenchmarkClientGetEndToEnd10TCP(b *testing.B) {
	b.Run("io_uring", func(b *testing.B) {
		benchmarkClientGetEndToEndTCP(b, 10, func(addr string) (net.Conn, error) {
			dialer := mynet.NewUringDialer()
			return dialer.DialContext(context.TODO(), "tcp", addr)
		})
	})

	b.Run("net", func(b *testing.B) {
		benchmarkClientGetEndToEndTCP(b, 10, func(addr string) (net.Conn, error) {
			return net.Dial("tcp", addr)
		})
	})

}
*/

func BenchmarkClientGetEndToEnd100TCP(b *testing.B) {
	/*
	b.Run("io_uring", func(b *testing.B) {
		benchmarkClientGetEndToEndTCPNoKeepAlive(b, 100, func(addr string) (net.Conn, error) {
			dialer := mynet.NewUringDialer()
			return dialer.DialContext(context.TODO(), "tcp", addr)
		})
	})
	*/

	time.Sleep(5 * time.Second)

	b.Run("net", func(b *testing.B) {
		benchmarkClientGetEndToEndTCPNoKeepAlive(b, 100, func(addr string) (net.Conn, error) {
			return net.Dial("tcp", addr)
		})
	})

}

func BenchmarkClientGetEndToEnd512TCP(b *testing.B) {
	/*
	b.Run("io_uring", func(b *testing.B) {
		benchmarkClientGetEndToEndTCPNoKeepAlive(b, 512, func(addr string) (net.Conn, error) {
			dialer := mynet.NewUringDialer()
			return dialer.DialContext(context.TODO(), "tcp", addr)
		})
	})
	*/

	time.Sleep(5 * time.Second)

	b.Run("net", func(b *testing.B) {
		benchmarkClientGetEndToEndTCPNoKeepAlive(b, 512, func(addr string) (net.Conn, error) {
			return net.Dial("tcp", addr)
		})
	})

}

func BenchmarkClientGetEndToEnd600TCP(b *testing.B) {
	/*
	b.Run("io_uring", func(b *testing.B) {
		benchmarkClientGetEndToEndTCPNoKeepAlive(b, 600, func(addr string) (net.Conn, error) {
			dialer := mynet.NewUringDialer()
			return dialer.DialContext(context.TODO(), "tcp", addr)
		})
	})
	*/

	time.Sleep(5 * time.Second)

	b.Run("net", func(b *testing.B) {
		benchmarkClientGetEndToEndTCPNoKeepAlive(b, 600, func(addr string) (net.Conn, error) {
			return net.Dial("tcp", addr)
		})
	})

}

func benchmarkClientGetEndToEndTCP(b *testing.B, parallelism int, dial DialFunc) {
	addr := "127.0.0.1:8543"

	ln, err := net.Listen("tcp4", addr)
	if err != nil {
		b.Fatalf("cannot listen %q: %v", addr, err)
	}

	ch := make(chan struct{})
	go func() {
		if err := Serve(ln, fasthttpEchoHandler); err != nil {
			b.Errorf("error when serving requests: %v", err)
		}
		close(ch)
	}()

	c := &Client{
		MaxConnsPerHost: runtime.GOMAXPROCS(-1) * parallelism,
		Dial:            dial,
	}

	requestURI := "/foo/bar?baz=123"
	url := "http://" + addr + requestURI
	b.SetParallelism(parallelism)
	b.RunParallel(func(pb *testing.PB) {
		var buf []byte
		for pb.Next() {
			statusCode, body, err := c.Get(buf, url)
			if err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
			if statusCode != StatusOK {
				b.Fatalf("unexpected status code: %d. Expecting %d", statusCode, StatusOK)
			}
			if string(body) != requestURI {
				b.Fatalf("unexpected response %q. Expecting %q", body, requestURI)
			}
			buf = body
		}
	})

	ln.Close()
	select {
	case <-ch:
	case <-time.After(time.Second):
		b.Fatalf("server wasn't stopped")
	}
}

func benchmarkClientGetEndToEndTCPNoKeepAlive(b *testing.B, parallelism int, dial DialFunc) {
	addr := "127.0.0.1:8543"

	ln, err := net.Listen("tcp4", addr)
	if err != nil {
		b.Fatalf("cannot listen %q: %v", addr, err)
	}

	ch := make(chan struct{})
	go func() {
		// Серверу не нужны изменения, он будет следовать указаниям клиента
		if err := Serve(ln, fasthttpEchoHandler); err != nil {
			b.Errorf("error when serving requests: %v", err)
		}
		close(ch)
	}()

	// Клиенту теперь не нужен большой пул соединений, но оставим его,
	// т.к. fasthttp все равно управляет ресурсами.
	c := &Client{
		// MaxConnsPerHost больше не играет роли для переиспользования,
		// но все еще ограничивает количество одновременных запросов в полете.
		MaxConnsPerHost: runtime.GOMAXPROCS(-1) * parallelism,
		Dial:            dial,
	}

	requestURI := "/foo/bar?baz=123"
	url := "http://" + addr + requestURI
	b.SetParallelism(parallelism)

	// Сбрасываем таймер, чтобы подготовка не влияла на результат
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		// Используем Acquire/Release для переиспользования объектов запроса/ответа,
		// что является идиомой в fasthttp для высокой производительности.
		req := AcquireRequest()
		defer ReleaseRequest(req) // Убедимся, что объект вернется в пул при выходе из горутины

		resp := AcquireResponse()
		defer ReleaseResponse(resp) // То же самое для ответа

		req.SetRequestURI(url)

		// !!! КЛЮЧЕВОЕ ИЗМЕНЕНИЕ !!!
		// Этот заголовок заставляет и клиента, и сервер закрыть соединение после ответа.
		req.Header.Set("Connection", "close")

		for pb.Next() {
			// Выполняем запрос с помощью c.Do
			if err := c.Do(req, resp); err != nil {
				b.Fatalf("unexpected error: %v", err)
			}

			statusCode := resp.StatusCode()
			body := resp.Body()

			if statusCode != StatusOK {
				b.Fatalf("unexpected status code: %d. Expecting %d", statusCode, StatusOK)
			}
			if string(body) != requestURI {
				b.Fatalf("unexpected response %q. Expecting %q", body, requestURI)
			}

			// Очищаем тело ответа, чтобы объект resp можно было переиспользовать в следующей итерации
			resp.ResetBody()
		}
	})

	ln.Close()
	select {
	case <-ch:
	case <-time.After(time.Second):
		b.Fatalf("server wasn't stopped")
	}
}

func benchmarkNetHTTPClientGetEndToEndTCP(b *testing.B, parallelism int) {
	addr := "127.0.0.1:8542"

	ln, err := net.Listen("tcp4", addr)
	if err != nil {
		b.Fatalf("cannot listen %q: %v", addr, err)
	}

	ch := make(chan struct{})
	go func() {
		if err := http.Serve(ln, http.HandlerFunc(nethttpEchoHandler)); err != nil && !strings.Contains(
			err.Error(), "use of closed network connection") {
			b.Errorf("error when serving requests: %v", err)
		}
		close(ch)
	}()

	c := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: parallelism * runtime.GOMAXPROCS(-1),
		},
	}

	requestURI := "/foo/bar?baz=123"
	url := "http://" + addr + requestURI
	b.SetParallelism(parallelism)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := c.Get(url)
			if err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				b.Fatalf("unexpected status code: %d. Expecting %d", resp.StatusCode, http.StatusOK)
			}
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				b.Fatalf("unexpected error when reading response body: %v", err)
			}
			if string(body) != requestURI {
				b.Fatalf("unexpected response %q. Expecting %q", body, requestURI)
			}
		}
	})

	ln.Close()
	select {
	case <-ch:
	case <-time.After(time.Second):
		b.Fatalf("server wasn't stopped")
	}
}

func benchmarkClientGetEndToEndInmemory(b *testing.B, parallelism int) {
	ln := fasthttputil.NewInmemoryListener()

	ch := make(chan struct{})
	go func() {
		if err := Serve(ln, fasthttpEchoHandler); err != nil {
			b.Errorf("error when serving requests: %v", err)
		}
		close(ch)
	}()

	c := &Client{
		MaxConnsPerHost: runtime.GOMAXPROCS(-1) * parallelism,
		Dial:            func(addr string) (net.Conn, error) { return ln.Dial() },
	}

	requestURI := "/foo/bar?baz=123"
	url := "http://unused.host" + requestURI
	b.SetParallelism(parallelism)
	b.RunParallel(func(pb *testing.PB) {
		var buf []byte
		for pb.Next() {
			statusCode, body, err := c.Get(buf, url)
			if err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
			if statusCode != StatusOK {
				b.Fatalf("unexpected status code: %d. Expecting %d", statusCode, StatusOK)
			}
			if string(body) != requestURI {
				b.Fatalf("unexpected response %q. Expecting %q", body, requestURI)
			}
			buf = body
		}
	})

	ln.Close()
	select {
	case <-ch:
	case <-time.After(time.Second):
		b.Fatalf("server wasn't stopped")
	}
}

func BenchmarkNetHTTPClientGetEndToEnd1Inmemory(b *testing.B) {
	benchmarkNetHTTPClientGetEndToEndInmemory(b, 1)
}

func BenchmarkNetHTTPClientGetEndToEnd10Inmemory(b *testing.B) {
	benchmarkNetHTTPClientGetEndToEndInmemory(b, 10)
}

func BenchmarkNetHTTPClientGetEndToEnd100Inmemory(b *testing.B) {
	benchmarkNetHTTPClientGetEndToEndInmemory(b, 100)
}

func BenchmarkNetHTTPClientGetEndToEnd1000Inmemory(b *testing.B) {
	benchmarkNetHTTPClientGetEndToEndInmemory(b, 1000)
}

func benchmarkNetHTTPClientGetEndToEndInmemory(b *testing.B, parallelism int) {
	ln := fasthttputil.NewInmemoryListener()

	ch := make(chan struct{})
	go func() {
		if err := http.Serve(ln, http.HandlerFunc(nethttpEchoHandler)); err != nil && !strings.Contains(
			err.Error(), "use of closed network connection") {
			b.Errorf("error when serving requests: %v", err)
		}
		close(ch)
	}()

	c := &http.Client{
		Transport: &http.Transport{
			Dial:                func(_, _ string) (net.Conn, error) { return ln.Dial() },
			MaxIdleConnsPerHost: parallelism * runtime.GOMAXPROCS(-1),
		},
	}

	requestURI := "/foo/bar?baz=123"
	url := "http://unused.host" + requestURI
	b.SetParallelism(parallelism)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := c.Get(url)
			if err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				b.Fatalf("unexpected status code: %d. Expecting %d", resp.StatusCode, http.StatusOK)
			}
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				b.Fatalf("unexpected error when reading response body: %v", err)
			}
			if string(body) != requestURI {
				b.Fatalf("unexpected response %q. Expecting %q", body, requestURI)
			}
		}
	})

	ln.Close()
	select {
	case <-ch:
	case <-time.After(time.Second):
		b.Fatalf("server wasn't stopped")
	}
}

func BenchmarkClientEndToEndBigResponse1Inmemory(b *testing.B) {
	benchmarkClientEndToEndBigResponseInmemory(b, 1)
}

func BenchmarkClientEndToEndBigResponse10Inmemory(b *testing.B) {
	benchmarkClientEndToEndBigResponseInmemory(b, 10)
}

func benchmarkClientEndToEndBigResponseInmemory(b *testing.B, parallelism int) {
	bigResponse := createFixedBody(1024 * 1024)
	h := func(ctx *RequestCtx) {
		ctx.SetContentType("text/plain")
		ctx.Write(bigResponse) //nolint:errcheck
	}

	ln := fasthttputil.NewInmemoryListener()

	ch := make(chan struct{})
	go func() {
		if err := Serve(ln, h); err != nil {
			b.Errorf("error when serving requests: %v", err)
		}
		close(ch)
	}()

	c := &Client{
		MaxConnsPerHost: runtime.GOMAXPROCS(-1) * parallelism,
		Dial:            func(addr string) (net.Conn, error) { return ln.Dial() },
	}

	requestURI := "/foo/bar?baz=123"
	url := "http://unused.host" + requestURI
	b.SetParallelism(parallelism)
	b.RunParallel(func(pb *testing.PB) {
		var req Request
		req.SetRequestURI(url)
		var resp Response
		for pb.Next() {
			if err := c.DoTimeout(&req, &resp, 5*time.Second); err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
			if resp.StatusCode() != StatusOK {
				b.Fatalf("unexpected status code: %d. Expecting %d", resp.StatusCode(), StatusOK)
			}
			body := resp.Body()
			if !bytes.Equal(bigResponse, body) {
				b.Fatalf("unexpected response %q. Expecting %q", body, bigResponse)
			}
		}
	})

	ln.Close()
	select {
	case <-ch:
	case <-time.After(time.Second):
		b.Fatalf("server wasn't stopped")
	}
}

func BenchmarkNetHTTPClientEndToEndBigResponse1Inmemory(b *testing.B) {
	benchmarkNetHTTPClientEndToEndBigResponseInmemory(b, 1)
}

func BenchmarkNetHTTPClientEndToEndBigResponse10Inmemory(b *testing.B) {
	benchmarkNetHTTPClientEndToEndBigResponseInmemory(b, 10)
}

func benchmarkNetHTTPClientEndToEndBigResponseInmemory(b *testing.B, parallelism int) {
	bigResponse := createFixedBody(1024 * 1024)
	h := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(HeaderContentType, "text/plain")
		w.Write(bigResponse) //nolint:errcheck
	}
	ln := fasthttputil.NewInmemoryListener()

	ch := make(chan struct{})
	go func() {
		if err := http.Serve(ln, http.HandlerFunc(h)); err != nil && !strings.Contains(
			err.Error(), "use of closed network connection") {
			b.Errorf("error when serving requests: %v", err)
		}
		close(ch)
	}()

	c := &http.Client{
		Transport: &http.Transport{
			Dial:                func(_, _ string) (net.Conn, error) { return ln.Dial() },
			MaxIdleConnsPerHost: parallelism * runtime.GOMAXPROCS(-1),
		},
		Timeout: 5 * time.Second,
	}

	requestURI := "/foo/bar?baz=123"
	url := "http://unused.host" + requestURI
	b.SetParallelism(parallelism)
	b.RunParallel(func(pb *testing.PB) {
		req, err := http.NewRequest(MethodGet, url, http.NoBody)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
		for pb.Next() {
			resp, err := c.Do(req)
			if err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				b.Fatalf("unexpected status code: %d. Expecting %d", resp.StatusCode, http.StatusOK)
			}
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				b.Fatalf("unexpected error when reading response body: %v", err)
			}
			if !bytes.Equal(bigResponse, body) {
				b.Fatalf("unexpected response %q. Expecting %q", body, bigResponse)
			}
		}
	})

	ln.Close()
	select {
	case <-ch:
	case <-time.After(time.Second):
		b.Fatalf("server wasn't stopped")
	}
}

func BenchmarkPipelineClient1(b *testing.B) {
	benchmarkPipelineClient(b, 1)
}

func BenchmarkPipelineClient10(b *testing.B) {
	benchmarkPipelineClient(b, 10)
}

func BenchmarkPipelineClient100(b *testing.B) {
	benchmarkPipelineClient(b, 100)
}

func BenchmarkPipelineClient1000(b *testing.B) {
	benchmarkPipelineClient(b, 1000)
}

func benchmarkPipelineClient(b *testing.B, parallelism int) {
	h := func(ctx *RequestCtx) {
		ctx.WriteString("foobar") //nolint:errcheck
	}
	ln := fasthttputil.NewInmemoryListener()

	ch := make(chan struct{})
	go func() {
		if err := Serve(ln, h); err != nil {
			b.Errorf("error when serving requests: %v", err)
		}
		close(ch)
	}()

	maxConns := runtime.GOMAXPROCS(-1)
	c := &PipelineClient{
		Dial:               func(addr string) (net.Conn, error) { return ln.Dial() },
		ReadBufferSize:     1024 * 1024,
		WriteBufferSize:    1024 * 1024,
		MaxConns:           maxConns,
		MaxPendingRequests: parallelism * maxConns,
	}

	requestURI := "/foo/bar?baz=123"
	url := "http://unused.host" + requestURI
	b.SetParallelism(parallelism)
	b.RunParallel(func(pb *testing.PB) {
		var req Request
		req.SetRequestURI(url)
		var resp Response
		for pb.Next() {
			if err := c.Do(&req, &resp); err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
			if resp.StatusCode() != StatusOK {
				b.Fatalf("unexpected status code: %d. Expecting %d", resp.StatusCode(), StatusOK)
			}
			body := resp.Body()
			if string(body) != "foobar" {
				b.Fatalf("unexpected response %q. Expecting %q", body, "foobar")
			}
		}
	})

	ln.Close()
	select {
	case <-ch:
	case <-time.After(time.Second):
		b.Fatalf("server wasn't stopped")
	}
}
