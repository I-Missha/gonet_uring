package ubatcher

import (
	"sync"
	"testing"
	"time"

	"github.com/godzie44/go-uring/uring"
)

func TestNewUBatcher(t *testing.T) {
	batcher := NewUBatcher(16)
	defer batcher.Close()

	if batcher == nil {
		t.Fatal("NewUBatcher returned nil")
	}

	batcher.Run()
	time.Sleep(1000 * time.Millisecond)
}

func TestUBatcher_Close(t *testing.T) {
	batcher := NewUBatcher(16)

	go batcher.Run()
	err := batcher.Close()
	if err != nil {
		t.Errorf("Close() failed: %v", err)
	}
}

func TestUBatcher_PushNopOperation(t *testing.T) {
	const batchSize = 16
	batcher := NewUBatcher(batchSize)
	defer batcher.Close()
	batcher.Run()

	var wg sync.WaitGroup
	actualBufferSize := cap(batcher.buffer.elements)
	for range actualBufferSize {
		wg.Add(1)
		go func() {
			ch := make(chan int32)
			cb := func(result int32, err error) {
				ch <- result
			}

			op := uring.Nop()

			batcher.PushOperaion(op, cb)
			<-ch
			wg.Done()
		}()
	}
	wg.Wait()

}
