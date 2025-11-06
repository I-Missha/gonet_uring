package ubalancer

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/godzie44/go-uring/uring"
)

func TestNewUBalancer(t *testing.T) {
	balancer := NewUBalancer(2, 8)

	if balancer.numBatchers != 2 {
		t.Errorf("Ожидалось 2 батчера, получено %d", balancer.numBatchers)
	}

	if len(balancer.batchers) != 2 {
		t.Errorf("Ожидалось 2 батчера в слайсе, получено %d", len(balancer.batchers))
	}

	// Проверяем что балансер не завершен
	if balancer.IsFinished() {
		t.Error("Балансер не должен быть завершен сразу после создания")
	}
}

func TestNewUBalancerDefaults(t *testing.T) {
	balancer := NewUBalancer(0, 0)

	if balancer.numBatchers != DefaultNumBatchers {
		t.Errorf("Ожидалось %d батчеров по умолчанию, получено %d", DefaultNumBatchers, balancer.numBatchers)
	}
}

func TestPushOperationDistribution(t *testing.T) {
	t.Log("Тест распределения операций по батчерам")
	balancer := NewUBalancer(3, 4)

	balancer.Run()
	time.Sleep(200 * time.Millisecond) // даем больше времени на запуск

	var completed int64
	callback := func(result int32, err error) {
		atomic.AddInt64(&completed, 1)
	}

	// Создаем простую no-op операцию
	op := uring.Nop()

	// Отправляем несколько операций
	for i := range 9 {
		cb := uint64(uintptr(unsafe.Pointer(&callback)))

		err := balancer.PushOperation(op, cb)
		if err != nil {
			t.Errorf("Ошибка при отправке операции %d: %v", i, err)
		}
	}

	// Проверяем, что counter увеличивается (round-robin работает)
	ticker := time.NewTicker(10000 * time.Millisecond)
	for atomic.LoadInt64(&completed) < 9 {
		select {
		case <-ticker.C:
			t.Errorf("Ожидалось минимум 9 операций, получено %d", completed)
		default:
			// продолжаем проверку
		}
	}

	t.Log("Операции отправлены\n")

	time.Sleep(200 * time.Millisecond)

	balancer.Close()
}

func TestBalancerLifecycle(t *testing.T) {
	balancer := NewUBalancer(2, 4)

	// Запуск
	balancer.Run()
	time.Sleep(50 * time.Millisecond)

	// Проверяем что балансер запущен но не завершен
	if balancer.IsFinished() {
		t.Error("Балансер не должен быть завершен сразу после запуска")
	}

	// Остановка - балансер завершится автоматически
	balancer.Shutdown()
	balancer.Wait()

	// Проверяем что балансер завершился
	if !balancer.IsFinished() {
		t.Error("Балансер должен быть завершен после ожидания")
	}

	// Закрытие
	err := balancer.Close()
	if err != nil {
		t.Errorf("Ошибка при закрытии балансировщика: %v", err)
	}
}

func TestConcurrentPushOperation(t *testing.T) {
	balancer := NewUBalancer(4, 8)
	balancer.Run()
	time.Sleep(200 * time.Millisecond) // даем больше времени на запуск

	var completed int64
	callback := func(result int32, err error) {
		atomic.AddInt64(&completed, 1)
	}

	op := uring.Nop()
	numGoroutines := 10
	operationsPerGoroutine := 20

	var wg sync.WaitGroup

	// Запускаем несколько горутин для параллельной отправки операций
	for i := range numGoroutines {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := range operationsPerGoroutine {
				cb := uint64(uintptr(unsafe.Pointer(&callback)))

				err := balancer.PushOperation(op, cb)
				if err != nil {
					t.Errorf("Горутина %d, операция %d: %v", goroutineID, j, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	time.Sleep(200 * time.Millisecond)

	// Проверяем, что counter увеличился на нужное количество
	expectedOps := uint64(numGoroutines * operationsPerGoroutine)
	if balancer.counter < expectedOps {
		t.Errorf("Ожидалось минимум %d операций, получено %d", expectedOps, balancer.counter)
	}

	balancer.Close()
}

func TestPushOperationErrors(t *testing.T) {
	balancer := NewUBalancer(2, 4)

	// Тест без запуска
	op := uring.Nop()
	callback := func(result int32, err error) {}

	err := balancer.PushOperation(op, uint64(uintptr(unsafe.Pointer(&callback))))
	if err != ErrNotRunning {
		t.Errorf("Ожидалась ошибка ErrNotRunning, получено: %v", err)
	}

	// Запускаем и сразу останавливаем
	balancer.Run()
	balancer.Shutdown()
	time.Sleep(10 * time.Millisecond)

	// Ждем завершения балансера
	balancer.Wait()

	// Тест после завершения
	err = balancer.PushOperation(op, uint64(uintptr(unsafe.Pointer(&callback))))
	if err != ErrShutdown {
		t.Errorf("Ожидалась ошибка ErrShutdown, получено: %v", err)
	}

	balancer.Close()
}

func TestAutoFinish(t *testing.T) {
	balancer := NewUBalancer(2, 4)

	// Запускаем балансер
	balancer.Run()
	time.Sleep(50 * time.Millisecond)

	// Проверяем что канал Done() не закрыт
	select {
	case <-balancer.Done():
		t.Error("Done() канал не должен быть закрыт до остановки")
	default:
		// ожидаемое поведение
	}

	// Останавливаем батчеры - балансер должен завершиться автоматически
	balancer.Shutdown()

	// Ждем сигнала завершения через канал
	select {
	case <-balancer.Done():
		// ожидаемое поведение
	case <-time.After(1 * time.Second):
		t.Error("Балансер не завершился автоматически в течение 1 секунды")
	}

	// Проверяем флаг завершения
	if !balancer.IsFinished() {
		t.Error("IsFinished() должен возвращать true после автоматического завершения")
	}
}

