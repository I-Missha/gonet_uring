package ubatcher

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/godzie44/go-uring/uring"
)

func TestUBatcherOperationCompletion(t *testing.T) {
	// Создаем UBatcher
	batcher := NewUBatcher(64)

	// Запускаем обработчик событий
	batcher.Run()
	defer batcher.Close()

	// Даем время на запуск обработчика
	time.Sleep(100 * time.Millisecond)

	// Канал для ожидания завершения операции
	done := make(chan struct{})
	var result int32
	var callbackErr error

	// Создаем callback функцию
	callback := func(res int32, err error) {
		t.Logf("Callback вызван: result=%d, err=%v", res, err)
		result = res
		callbackErr = err
		close(done)
	}

	// Создаем простую no-op операцию
	operation := uring.Nop()

	// Конвертируем callback в указатель
	cbPtr := uint64(uintptr(unsafe.Pointer(&callback)))

	t.Logf("Добавляем операцию в батчер")
	// Добавляем операцию в батчер
	batcher.PushOperaion(operation, cbPtr)

	// Ожидаем завершения операции с таймаутом
	select {
	case <-done:
		t.Logf("Операция завершена: result=%d, err=%v", result, callbackErr)
		if callbackErr != nil {
			t.Errorf("Неожиданная ошибка в callback: %v", callbackErr)
		}
	case <-time.After(5 * time.Second):
		t.Error("Операция не завершилась в течение 5 секунд")
	}
}

func TestUBatcherMultipleOperations(t *testing.T) {
	batcher := NewUBatcher(16)
	batcher.Run()
	defer batcher.Close()

	// Даем время на запуск обработчика
	time.Sleep(100 * time.Millisecond)

	const numOperations = 10
	var completed int64
	var wg sync.WaitGroup

	wg.Add(numOperations)

	// Запускаем несколько операций
	for i := 0; i < numOperations; i++ {
		callback := func(result int32, err error) {
			count := atomic.AddInt64(&completed, 1)
			t.Logf("Операция завершена #%d: result=%d, err=%v", count, result, err)
			wg.Done()
		}

		operation := uring.Nop()
		cbPtr := uint64(uintptr(unsafe.Pointer(&callback)))

		t.Logf("Добавляем операцию #%d", i+1)
		batcher.PushOperaion(operation, cbPtr)
	}

	// Ждем завершения всех операций
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if atomic.LoadInt64(&completed) != numOperations {
			t.Errorf("Ожидалось %d завершенных операций, получено %d", numOperations, completed)
		}
		t.Logf("Все %d операций успешно завершены", numOperations)
	case <-time.After(10 * time.Second):
		t.Errorf("Не все операции завершились в течение 10 секунд. Завершено: %d из %d",
			atomic.LoadInt64(&completed), numOperations)
	}
}
