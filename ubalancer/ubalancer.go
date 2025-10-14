package ubalancer

import (
	"errors"
	"log"
	"sync"
	"sync/atomic"

	"github.com/I-Missha/gonet_uring/ubatcher"
	"github.com/godzie44/go-uring/uring"
)

// UBalancer распределяет операции между несколькими UBatcher'ами.
// Thread-safe для использования из множественных горутин.
// Автоматически завершается когда все батчеры завершают работу.
type UBalancer struct {
	batchers    []*ubatcher.UBatcher
	numBatchers int
	counter     uint64 // для round-robin балансировки (atomic)
	shutdown    int64  // atomic
	running     int64  // atomic - флаг запуска
	finished    int64  // atomic - флаг завершения
	wg          sync.WaitGroup // для горутин батчеров
	done        chan struct{} // сигнал завершения балансера
}

const (
	DefaultNumBatchers = 4
	DefaultBatchSize   = 16
)

var (
	ErrNotRunning = errors.New("балансировщик не запущен")
	ErrShutdown   = errors.New("балансировщик остановлен")
)

func NewUBalancer(numBatchers int, batchSize uint32) *UBalancer {
	if numBatchers <= 0 {
		numBatchers = DefaultNumBatchers
	}

	if batchSize == 0 {
		batchSize = DefaultBatchSize
	}

	batchers := make([]*ubatcher.UBatcher, numBatchers)
	for i := 0; i < numBatchers; i++ {
		batchers[i] = ubatcher.NewUBatcher(batchSize)
	}

	log.Printf("[UBalancer] Создан балансировщик с %d батчерами", numBatchers)
	return &UBalancer{
		batchers:    batchers,
		numBatchers: numBatchers,
		counter:     0,
		done:        make(chan struct{}),
	}
}

// PushOperation распределяет операцию между батчерами по round-robin.
// Thread-safe для использования из любых горутин.
func (ub *UBalancer) PushOperation(operation uring.Operation, cb func(result int32, err error)) error {
	// Проверяем состояние балансировщика
	if atomic.LoadInt64(&ub.finished) == 1 {
		return ErrShutdown
	}
	if atomic.LoadInt64(&ub.running) == 0 {
		return ErrNotRunning
	}

	// Round-robin выбор батчера (thread-safe)
	idx := atomic.AddUint64(&ub.counter, 1) % uint64(ub.numBatchers)
	batcher := ub.batchers[idx]

	batcher.PushOperaion(operation, cb)
	return nil
}

// Run запускает все батчеры.
// После вызова балансировщик готов к приему операций из любых горутин.
// Балансер автоматически завершится когда все батчеры завершат работу.
func (ub *UBalancer) Run() {
	// Атомарно устанавливаем флаг запуска
	if !atomic.CompareAndSwapInt64(&ub.running, 0, 1) {
		log.Printf("[UBalancer] Балансировщик уже запущен")
		return
	}

	log.Printf("[UBalancer] Запуск всех батчеров")
	for i, batcher := range ub.batchers {
		log.Printf("[UBalancer] Запуск батчера %d", i)
		batcher.Run()
	}

	// Запускаем горутину-монитор для автоматического завершения
	go ub.monitor()
}

// Shutdown инициирует остановку всех батчеров.
// Thread-safe для вызова из любых горутин.
// После остановки балансер автоматически завершится.
func (ub *UBalancer) Shutdown() {
	// Атомарно устанавливаем флаг остановки
	if !atomic.CompareAndSwapInt64(&ub.shutdown, 0, 1) {
		log.Printf("[UBalancer] Остановка уже инициирована")
		return
	}

	log.Printf("[UBalancer] Инициирована остановка")
	
	for i, batcher := range ub.batchers {
		log.Printf("[UBalancer] Остановка батчера %d", i)
		batcher.Shutdown()
	}
}

// monitor отслеживает завершение всех батчеров и сигнализирует о завершении балансера
func (ub *UBalancer) monitor() {
	log.Printf("[UBalancer] Запущен монитор завершения")
	
	// Ждем завершения всех батчеров
	for i, batcher := range ub.batchers {
		log.Printf("[UBalancer] Ожидание завершения батчера %d", i)
		batcher.Wait()
		log.Printf("[UBalancer] Батчер %d завершен", i)
	}
	
	log.Printf("[UBalancer] Все батчеры завершили работу")
	
	// Устанавливаем флаг завершения
	atomic.StoreInt64(&ub.finished, 1)
	
	// Сигнализируем о завершении
	close(ub.done)
	
	log.Printf("[UBalancer] Балансировщик завершен")
}

// Wait ожидает автоматического завершения балансера
func (ub *UBalancer) Wait() {
	log.Printf("[UBalancer] Ожидание завершения балансера")
	<-ub.done
}

// Close инициирует остановку и ждет завершения всех батчеров
func (ub *UBalancer) Close() error {
	ub.Shutdown()
	ub.Wait()

	log.Printf("[UBalancer] Закрытие всех батчеров")
	var lastErr error
	for i, batcher := range ub.batchers {
		if err := batcher.Close(); err != nil {
			log.Printf("[UBalancer] Ошибка при закрытии батчера %d: %v", i, err)
			lastErr = err
		}
	}

	return lastErr
}

// Done возвращает канал, который закрывается при завершении балансера
func (ub *UBalancer) Done() <-chan struct{} {
	return ub.done
}

// IsFinished возвращает true если балансер завершил работу
func (ub *UBalancer) IsFinished() bool {
	return atomic.LoadInt64(&ub.finished) == 1
}


