package net

import (
	"context"
	"fmt"
	gonet "net"
	"sync"
	"testing"
	"time"

	"github.com/valyala/fasthttp"
)

// startTestServer запускает простой fasthttp сервер и возвращает jego адрес.
func startTestServer(b *testing.B) string {
	ln, err := gonet.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		b.Fatalf("net.Listen failed: %v", err)
	}

	server := &fasthttp.Server{
		Handler: func(ctx *fasthttp.RequestCtx) {
			ctx.SetConnectionClose()
			fmt.Fprint(ctx, "ok")
		},
		// Отключаем логгирование для чистоты бенчмарка
		Logger: nil,
	}

	// Запускаем сервер в фоновой горутине
	go func() {
		if err := server.Serve(ln); err != nil {
			// Ошибки здесь обычно возникают при закрытии listener'a,
			// поэтому мы можем их игнорировать в контексте теста.
		}
	}()

	// При завершении теста останавливаем сервер и listener
	b.Cleanup(func() {
		ln.Close()
		server.Shutdown()
	})

	return ln.Addr().String()
}

func BenchmarkCustomDialer(b *testing.B) {
	serverAddr := startTestServer(b)

	dialer := NewUringDialer() // Создаем новый диалер

	// Количество одновременных соединений
	numConnections := 10000

	b.ResetTimer()

	// Открываем много соединений одновременно
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for i := 0; i < numConnections; i++ {
		wg.Add(1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					b.Errorf("Recovered from panic: %v", r)
				}
			}()
			defer wg.Done()

			// Создаем соединение через твой диалер
			conn, err := dialer.DialContext(ctx, "tcp", serverAddr)
			if err != nil {
				b.Errorf("DialContext failed: %v", err)
				return
			}
			defer conn.Close()

			// Отправляем простой HTTP-запрос
			_, err = conn.Write([]byte("GET /test HTTP/1.1\r\nHost: " + serverAddr + "\r\n\r\n"))
			if err != nil {
				b.Errorf("Write failed: %v", err)
				return
			}

			// Читаем ответ
			buffer := make([]byte, 1024)
			_, err = conn.Read(buffer)
			if err != nil {
				b.Errorf("Read failed: %v", err)
			}
		}()
	}

	// Ждем завершения всех соединений
	wg.Wait()

	b.StopTimer()
	b.Logf("Total successful dials: %d", dialer.GetSuccessCount())
}
