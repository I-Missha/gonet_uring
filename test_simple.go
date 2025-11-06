package main

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/godzie44/go-uring/uring"
)

func main() {
	ring, err := uring.New(256)
	if err != nil {
		panic(err)
	}
	defer ring.Close()

	var completed bool
	callback := func(result int32, err error) {
		fmt.Printf("Callback: result=%d, err=%v\n", result, err)
		completed = true
	}

	// Добавляем nop операцию
	op := uring.Nop()
	userData := uint64(uintptr(unsafe.Pointer(&callback)))

	err = ring.QueueSQE(op, 0, userData)
	if err != nil {
		panic(err)
	}


	// Ждём завершения
	start := time.Now()
	for !completed && time.Since(start) < 5*time.Second {
		cqes := make([]*uring.CQEvent, 1)
		n := ring.PeekCQEventBatch(cqes)

		if n > 0 {
			fmt.Printf("Got %d CQE events\n", n)
			for i := 0; i < n; i++ {
				cqe := cqes[i]
				result := cqe.Res
				err := cqe.Error()
				ptr := unsafe.Pointer(uintptr(cqe.UserData))
				cb := *(*func(int32, error))(ptr)

				fmt.Printf("Processing CQE: result=%d, err=%v\n", result, err)
				cb(result, err)
			}
			ring.AdvanceCQ(uint32(n))
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	if !completed {
		fmt.Println("Operation did not complete within timeout")
	}
}
