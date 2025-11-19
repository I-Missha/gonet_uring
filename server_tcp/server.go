package main

import (
	"fmt"
	"log"
	"net" // Импортируем стандартный пакет net
	time "time"

)

func handleConnection(conn net.Conn) {
	defer conn.Close() // Сервер закрывает соединение

	fmt.Printf("Received connection from %s\n", conn.RemoteAddr().String())

	// Читаем данные от клиента
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Printf("Read error: %s", err)
		return
	}

	// Отвечаем
	_, err = conn.Write([]byte("Hello from server!\n"))
	if err != nil {
		log.Printf("Write error: %s", err)
		return
	}

	fmt.Printf("Received: %s\n", string(buf[:n]))

	// ВАЖНО: Сервер инициирует закрытие
	// Добавляем небольшую задержку чтобы клиент успел прочитать
	time.Sleep(10 * time.Millisecond)
}

func main() {

	basePort := 9543
	addr := fmt.Sprintf("[::]:%d", basePort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to create listener: %s", err)
	}

	fmt.Printf("Starting dual-stack echo server on %s\n", ln.Addr().String())

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatalf("Failed to accept connection: %s", err)
		}
		go handleConnection(conn)
	}

}
