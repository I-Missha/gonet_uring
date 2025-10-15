package main

import (
	"fmt"
	"log"
	"net"
)

func main() {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal("Ошибка запуска сервера:", err)
	}
	defer listener.Close()

	fmt.Println("Эхо-сервер запущен на :8080")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Ошибка принятия соединения:", err)
			continue
		}

		go func(c net.Conn) {
			defer c.Close()
			buffer := make([]byte, 1024)
			for {
				n, err := c.Read(buffer)
				if err != nil {
					return
				}
				if n > 0 {
					c.Write(buffer[:n])
				}
			}
		}(conn)
	}
}