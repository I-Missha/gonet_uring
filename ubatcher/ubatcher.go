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
	DefaultBatchSize = 16
	DefaultTimout    = 10 * time.Millisecond // after this time we will submit the batch independent of the number of elements in the batch

	BufferSizeMultiplier = 2
	RingSizeMultiplier   = 10
)

/*
the current logic of the program assumes the presence of a buffer larger than the batch, so as not to block the goroutines while waiting for free space in the batch, in addition,
the very concept of the buffer assumes rings larger than the buffer itself, because the ring may overflow, which is a critical situation

idk about this
todo: think
*/
func NewUBatcher(size uint32) *UBatcher {
	ringSize := size * RingSizeMultiplier
	bufferSize := uint32(DefaultBatchSize * BufferSizeMultiplier)

	ring, err := uring.New(ringSize)
	if err != nil {
		panic(err)
	}

	return &UBatcher{
		ring:        ring,
		callbacks:   make(map[uint64]callback),
		nextID:      rand.Uint64(),
		callbackMut: sync.Mutex{},
		buffer:      NewBuffer(bufferSize),
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
		u.batchSignal <- struct{}{}
	}
}

func (u *UBatcher) CQEventsHandlerRun() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(u.cqDone)

	for {
		// Проверяем сигнал остановки
		if atomic.LoadInt64(&u.shutdown) == 1 {
			return
		}

		// Ожидаем завершения операций с тайм-аутом
		cqe, err := u.ring.WaitCQEventsWithTimeout(1, 500*time.Millisecond)
		if err != nil {
			continue
		}

		u.processCQE(cqe)
		u.ring.SeenCQE(cqe)
	}
}

func (u *UBatcher) processCQE(cqe *uring.CQEvent) {
	u.callbackMut.Lock()
	defer u.callbackMut.Unlock()

	result := cqe.Res
	err := cqe.Error()

	if cb, exists := u.callbacks[cqe.UserData]; exists {
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

func (u *UBatcher) Shutdown() {
	atomic.StoreInt64(&u.shutdown, 1)
}

func (u *UBatcher) Wait() {
	<-u.cqDone
	<-u.sqDone
}

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

	for _, e := range events {
		u.addToUring(e.operation, e.cb)
	}

	_, err := u.ring.Submit()
	if err != nil {
		log.Printf("[UBatcher] Ошибка отправки операций в uring: %v", err)
	}
}

func (u *UBatcher) SQEventsHandlerRun() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(u.sqDone)

	timer := time.NewTimer(DefaultTimout)
	defer timer.Stop()

	for {
		// Проверяем сигнал остановки
		if atomic.LoadInt64(&u.shutdown) == 1 {
			return
		}

		select {
		case <-u.batchSignal:
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
