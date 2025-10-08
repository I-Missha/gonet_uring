package main

import (
	"io"
	"log"
	"net"
	"time"
	unet "github.com/I-Missha/gonet_uring/net"
)

func main() {
	// Запускаем простой эхо-сервер в отдельной горутине
	go startEchoServer()
	time.Sleep(100 * time.Millisecond) // Даем серверу время запуститься

	// Клиент
	log.Println("Client: connecting to server...")
	stdConn, err := net.Dial("tcp", "127.0.0.1:8080")
	if err != nil {
		log.Fatalf("Client: failed to dial: %v", err)
	}

	// Оборачиваем стандартное соединение в наш UringConn
	// Мы должны привести тип к *net.TCPConn
	tcpConn, ok := stdConn.(*net.TCPConn)
	if !ok {
		log.Fatalf("Failed to cast to *net.TCPConn")
	}

	log.Println("Client: wrapping connection with io_uring...")
	uringConn, err := unet.NewUringConn(tcpConn)
	if err != nil {
		log.Fatalf("Client: failed to create uring conn: %v", err)
	}
	defer uringConn.Close()

	log.Println("Client: UringConn created. Remote addr:", uringConn.RemoteAddr())

	// Используем наш UringConn как обычный net.Conn
	message := "Hello, io_uring!"
	log.Printf("Client: writing message: '%s'", message)
	n, err := uringConn.Write([]byte(message))
	if err != nil {
		log.Fatalf("Client: write failed: %v", err)
	}
	log.Printf("Client: wrote %d bytes", n)

	// Читаем ответ от эхо-сервера
	buffer := make([]byte, 1024)
	n, err = uringConn.Read(buffer)
	if err != nil {
		log.Fatalf("Client: read failed: %v", err)
	}
	log.Printf("Client: read %d bytes. Response: '%s'", n, string(buffer[:n]))

	if string(buffer[:n]) == message {
		log.Println("SUCCESS: Received message matches sent message.")
	} else {
		log.Println("FAILURE: Mismatch in messages.")
	}
}

// startEchoServer запускает TCP сервер, который просто возвращает обратно все, что получает.
func startEchoServer() {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("Server: failed to listen: %v", err)
	}
	defer listener.Close()
	log.Println("Server: listening on :8080")

	conn, err := listener.Accept()
	if err != nil {
		log.Printf("Server: failed to accept connection: %v", err)
		return
	}
	defer conn.Close()
	log.Println("Server: accepted connection from", conn.RemoteAddr())

	// Простой эхо-цикл
	io.Copy(conn, conn)
	log.Println("Server: connection closed.")
}

