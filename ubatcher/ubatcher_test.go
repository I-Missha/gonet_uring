package ubatcher

import (
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

func TestUBatcher_PushOperation(t *testing.T) {
	batcher := NewUBatcher(16)
	defer batcher.Close()

	cb := func(result int32, err error) {
		// Колбек для теста
	}

	// Создаём фиктивную операцию
	op := uring.Nop()

	// Добавляем операцию
	batcher.PushOperaion(op, cb)

	// Проверяем что операция добавилась в буфер
	if len(batcher.buffer.elements) != 1 {
		t.Errorf("Expected buffer size 1, got %d", len(batcher.buffer.elements))
	}
}

func TestUBatcher_PushOperation_BatchSignal(t *testing.T) {
	batcher := NewUBatcher(16)
	defer batcher.Close()

	// Устанавливаем маленький размер батча для теста
	batcher.batchSize = 2

	cb := func(result int32, err error) {}
	op := uring.Nop()

	batcher.PushOperaion(op, cb)

	batcher.PushOperaion(op, cb)

}

