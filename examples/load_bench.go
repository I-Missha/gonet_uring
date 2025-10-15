package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/I-Missha/gonet_uring/net"
)

func main() {
	const (
		targetRPS     = 3500
		durationSecs  = 30
		targetAddr    = "127.0.0.1:8080"
	)

	dialer := net.NewUringDialer()
	
	var (
		totalConnections int64
		successConnections int64
		failedConnections int64
		wg sync.WaitGroup
	)

	fmt.Printf("Запуск нагрузочного теста: %d соединений/сек в течение %d секунд\n", targetRPS, durationSecs)
	fmt.Printf("Цель: %s\n", targetAddr)
	
	startTime := time.Now()
	ticker := time.NewTicker(time.Second / time.Duration(targetRPS))
	defer ticker.Stop()

	timeout := time.After(time.Duration(durationSecs) * time.Second)

	done := make(chan struct{})
	
	go func() {
		defer close(done)
		for {
			select {
			case <-ticker.C:
				atomic.AddInt64(&totalConnections, 1)
				wg.Add(1)
				
				go func() {
					defer wg.Done()
					
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
					
					conn, err := dialer.DialContext(ctx, "tcp", targetAddr)
					if err != nil {
						atomic.AddInt64(&failedConnections, 1)
						return
					}
					
					atomic.AddInt64(&successConnections, 1)
					conn.Close()
				}()
				
			case <-timeout:
				ticker.Stop()
				return
			}
		}
	}()

	statsTicker := time.NewTicker(time.Second)
	defer statsTicker.Stop()

	go func() {
		for {
			select {
			case <-statsTicker.C:
				elapsed := time.Since(startTime).Seconds()
				total := atomic.LoadInt64(&totalConnections)
				success := atomic.LoadInt64(&successConnections)
				failed := atomic.LoadInt64(&failedConnections)
				
				fmt.Printf("Время: %.1fs | Всего: %d | Успешно: %d | Ошибок: %d | RPS: %.1f\n", 
					elapsed, total, success, failed, float64(total)/elapsed)
				
			case <-timeout:
				return
			}
		}
	}()

	<-timeout
	fmt.Println("Ожидание завершения всех соединений...")
	
	waitDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitDone)
	}()
	
	select {
	case <-waitDone:
		fmt.Println("Все соединения завершены")
	case <-time.After(10 * time.Second):
		fmt.Println("Таймаут ожидания завершения соединений")
	}

	elapsed := time.Since(startTime)
	total := atomic.LoadInt64(&totalConnections)
	success := atomic.LoadInt64(&successConnections)
	failed := atomic.LoadInt64(&failedConnections)

	fmt.Printf("\n=== РЕЗУЛЬТАТЫ ===\n")
	fmt.Printf("Общее время: %.2fs\n", elapsed.Seconds())
	fmt.Printf("Всего попыток соединений: %d\n", total)
	fmt.Printf("Успешных соединений: %d\n", success)
	fmt.Printf("Неудачных соединений: %d\n", failed)
	fmt.Printf("Средний RPS: %.2f\n", float64(total)/elapsed.Seconds())
	fmt.Printf("Успешность: %.2f%%\n", float64(success)/float64(total)*100)
}