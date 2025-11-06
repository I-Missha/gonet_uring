package main

import (
	"fmt"
	"log"
	"net" // Импортируем стандартный пакет net

	"github.com/valyala/fasthttp"
)

func echoHandler(ctx *fasthttp.RequestCtx) {
	fmt.Printf("Received request from %s\n", ctx.RemoteAddr())
	ctx.SetContentType("text/plain; charset=utf-8")
	ctx.Write(ctx.RequestURI())
}

func main() {
	addr := "[::]:8543"

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to create listener: %s", err)
	}

	fmt.Printf("Starting dual-stack echo server on %s\n", ln.Addr().String())

	if err := fasthttp.Serve(ln, echoHandler); err != nil {
		log.Fatalf("Error in fasthttp.Serve: %s", err)
	}
}
