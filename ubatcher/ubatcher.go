package ubatcher

import (
	"log"
	"math/rand/v2"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/godzie44/go-uring/uring"
)

type callback func(result int32, err error)

// inside of uring event resutl we have user data, user data is request id and after getting cqe we can call the corresponding callback
// so, there will be cqe event handler

type UBatcher struct {
	ring *uring.Ring

	callbackMut sync.Mutex
	callbacks   map[uint64]callback
	nextID      uint64

	buffer      *Buffer
	batchSignal chan struct{}
	batchSize   uint32

	// Для управления обработчиками
	shutdown int64 // atomic
	cqDone   chan struct{}
	sqDone   chan struct{}
}

const (
	DefaultBatchSize  = 16
	DefaultBufferSize = DefaultBatchSize + 10 // inside of the buffer is mutex placed, so to avoid lock due to full batch we need some extra space
	DefaultTimout     = 10 * time.Millisecond // after this time we will submit the batch independent of the number of elements in the batch
)

func NewUBatcher(size uint32) *UBatcher {
	ring, err := uring.New(size)
	if err != nil {
		panic(err)
	}

	log.Printf("[UBatcher] Создан новый UBatcher с размером ring=%d, batchSize=%d", size, DefaultBatchSize)
	return &UBatcher{
		ring:        ring,
		callbacks:   make(map[uint64]callback),
		nextID:      rand.Uint64(),
		callbackMut: sync.Mutex{},
		buffer:      NewBuffer(DefaultBufferSize),
		batchSignal: make(chan struct{}, 1),
		batchSize:   DefaultBatchSize,
		cqDone:      make(chan struct{}),
		sqDone:      make(chan struct{}),
	}
}

func (u *UBatcher) addToUring(operation uring.Operation, cb callback) {
	u.callbackMut.Lock()
	defer u.callbackMut.Unlock()

	userData := u.nextID
	u.nextID++
	// shitty code :)
	var err error
	for range 4 {
		u.callbacks[userData] = cb
		err = u.ring.QueueSQE(operation, 0, userData) // note: NextSQE is used inside of QueueSQE, NextSQE returns only one error = ErrSQOverflow

		if err == nil {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	if err != nil {
		panic(err)
	}
}

func (u *UBatcher) PushOperaion(operation uring.Operation, cb callback) {
	entry := Entry{
		operation: operation,
		cb:        cb,
	}

	u.buffer.Put(&entry)

	if len(u.buffer.elements) >= int(u.batchSize) { // redo this moment :D
		log.Printf("[UBatcher] батч можно отправлять (%d), отправляем сигнал", u.batchSize)
		u.batchSignal <- struct{}{}
	}
}

// CQEventsHandlerRun запускает обработчик событий завершения (CQE)
// Должен выполняться в отдельной горутине
func (u *UBatcher) CQEventsHandlerRun() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(u.cqDone)

	log.Printf("[UBatcher] Запущен обработчик CQE событий")
	for {
		// Проверяем сигнал остановки
		if atomic.LoadInt64(&u.shutdown) == 1 {
			log.Printf("[UBatcher] Получен сигнал остановки, завершаем обработчик CQE")
			return
		}

		// Ожидаем завершения операций с тайм-аутом
		cqe, err := u.ring.WaitCQEventsWithTimeout(1, 500*time.Millisecond)
		if err != nil {
			continue
		}

		u.processCQE(cqe)
		// Отмечаем событие как обработанное
		u.ring.SeenCQE(cqe)
	}
}

// processCQE обрабатывает одно событие завершения
func (u *UBatcher) processCQE(cqe *uring.CQEvent) {
	u.callbackMut.Lock()
	defer u.callbackMut.Unlock()

	result := cqe.Res
	err := cqe.Error()

	if cb, exists := u.callbacks[cqe.UserData]; exists {
		log.Printf("[UBatcher] Обработка CQE: userData=%d, result=%d, err=%v", cqe.UserData, result, err)
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[UBatcher] Паника в колбеке userData=%d: %v", cqe.UserData, r)
				}
			}()
			cb(result, err)
		}()
		delete(u.callbacks, cqe.UserData)
	} else {
		log.Printf("[UBatcher] Колбек не найден для userData=%d", cqe.UserData)
	}
}

// Shutdown останавливает обработчик CQE
func (u *UBatcher) Shutdown() {
	atomic.StoreInt64(&u.shutdown, 1)
}

// Wait ожидает завершения обоих обработчиков
func (u *UBatcher) Wait() {
	<-u.cqDone
	<-u.sqDone
}

// Close закрывает UBatcher и освобождает ресурсы
func (u *UBatcher) Close() error {
	u.Shutdown()
	u.Wait()
	return u.ring.Close()
}

type Entry struct {
	operation uring.Operation
	cb        callback
}

func handleBatch(u *UBatcher) {
	events := u.buffer.GetAll()
	if len(events) == 0 {
		return
	}

	log.Printf("[UBatcher] Обработка батча из %d операций", len(events))
	for _, e := range events {
		u.addToUring(e.operation, e.cb)
	}

	submitted, err := u.ring.Submit()
	log.Printf("[UBatcher] Отправлено %d операций в uring, err=%v", submitted, err)
}

func (u *UBatcher) SQEventsHandlerRun() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(u.sqDone)

	log.Printf("[UBatcher] Запущен основной цикл батчинга с таймаутом %v", DefaultTimout)
	timer := time.NewTimer(DefaultTimout)
	defer timer.Stop()

	for {
		// Проверяем сигнал остановки
		if atomic.LoadInt64(&u.shutdown) == 1 {
			log.Printf("[UBatcher] Получен сигнал остановки, завершаем SQ handler")
			return
		}

		select {
		case <-u.batchSignal:
			log.Printf("[UBatcher] Получен сигнал обработки батча")
			handleBatch(u)
			timer.Reset(DefaultTimout)

		case <-timer.C:
			handleBatch(u)
			timer.Reset(DefaultTimout)
		}
	}
}

func (u *UBatcher) Run() {
	go u.SQEventsHandlerRun()
	go u.CQEventsHandlerRun()
}
