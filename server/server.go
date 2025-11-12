package main

import (
	"fmt"
	"log"
	"net" // Импортируем стандартный пакет net
	"os"
	"time"

	"github.com/valyala/fasthttp"
)

var withSleep = false
func echoHandler(ctx *fasthttp.RequestCtx) {
	fmt.Printf("Received request from %s\n", ctx.RemoteAddr())
	ctx.SetContentType("text/plain; charset=utf-8")
	ctx.Write(ctx.RequestURI())
	if withSleep {
		time.Sleep(time.Millisecond * 50)
	}
}

func main() {
	args := os.Args
	if len(args) > 1 {
		withSleep = true
	}

	basePort := 8543
	for i := 0; i < 1000; i++ {
		addr := fmt.Sprintf("[::]:%d", basePort+i)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			log.Fatalf("Failed to create listener: %s", err)
		}

		fmt.Printf("Starting dual-stack echo server on %s\n", ln.Addr().String())

		if err := fasthttp.Serve(ln, echoHandler); err != nil {
			log.Fatalf("Error in fasthttp.Serve: %s", err)
		}
	}
}
