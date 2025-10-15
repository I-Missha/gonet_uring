package net

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestConn(t *testing.T) {
	// Запускаем HTTP echo server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Echo сервер - возвращаем тело запроса
		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		w.Write(body)
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Запускаем сервер в горутине
	go func() {
		server.ListenAndServe()
	}()

	// Ждем пока сервер запустится
	time.Sleep(100 * time.Millisecond)
	defer server.Shutdown(context.Background())

	// Подключаемся через наш UringDialer
	dialer := NewUringDialer()
	defer dialer.balancer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := dialer.DialContext(ctx, "tcp", "127.0.0.1:8080")
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	// Отправляем HTTP POST запрос
	testData := "Hello, World!"
	httpRequest := fmt.Sprintf("POST / HTTP/1.1\r\nHost: 127.0.0.1:8080\r\nContent-Length: %d\r\n\r\n%s", len(testData), testData)

	// Записываем данные
	n, err := conn.Write([]byte(httpRequest))
	if err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	if n != len(httpRequest) {
		t.Fatalf("Expected to write %d bytes, wrote %d", len(httpRequest), n)
	}

	// Читаем ответ
	response := make([]byte, 1024)
	n, err = conn.Read(response)
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	responseStr := string(response[:n])

	// Проверяем что в ответе есть наши данные
	if !contains(responseStr, testData) {
		t.Errorf("Response doesn't contain sent data. Sent: %q, Response: %q", testData, responseStr)
	}

	t.Logf("Successfully sent and received data through UringConn")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

