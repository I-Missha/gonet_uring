package ubatcher

import (
	"runtime"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/godzie44/go-uring/uring"
)

type callback func(result int32, err error)

type UBatcher struct {
	ring *uring.Ring
	lock int64 // atomic

	shutdown int64 // atomic
	cqDone   chan struct{}
}

const (
	RingSizeMultiplier = 1
)

func NewUBatcher(size uint32) *UBatcher {
	ringSize := size * RingSizeMultiplier

	ring, err := uring.New(ringSize, uring.WithSQPoll(100*time.Millisecond))

	if err != nil {
		panic(err)
	}

	return &UBatcher{
		ring:     ring,
		cqDone:   make(chan struct{}),
	}
}

func (u *UBatcher) Lock() {
	for !atomic.CompareAndSwapInt64(&u.lock, 0, 1){
		runtime.Gosched()
	}
}

func (u *UBatcher) Unlock() {
	atomic.StoreInt64(&u.lock, 0)
}

func (u *UBatcher) addToUring(operation uring.Operation, cb uint64) {
	userData := cb
	// shitty code :)
	var err error
	for range 10 {
		err = u.ring.QueueSQE(operation, 0, userData) // note: NextSQE is used inside of QueueSQE, NextSQE returns only one error = ErrSQOverflow

		if err == nil {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	if err != nil {
		panic(err)
	}

	// Submit the operation to the kernel
	_, err = u.ring.Submit()
	if err != nil {
		panic(err)
	}
}

func (u *UBatcher) PushOperaion(operation uring.Operation, cb uint64) {
	u.Lock()
	defer u.Unlock()
	u.addToUring(operation, cb)
}

const CQEventsToWait = 8

func (u *UBatcher) CQEventsHandlerRun() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	buff := make([]*uring.CQEvent, CQEventsToWait)
	defer close(u.cqDone)
	count := 0
	all := 0

	for {
		if atomic.LoadInt64(&u.shutdown) == 1 {
			return
		}

		// Если нет событий, ждем с таймаутом
		_, err := u.ring.WaitCQEventsWithTimeout(1, 100*time.Millisecond)
		if err != nil {
			// Таймаут или другая ошибка - просто продолжаем цикл
			continue
		}
		all++

		// Сначала проверяем есть ли готовые события
		u.processCQE(buff)
		count++
	}
}

func (u *UBatcher) processCQE(buff []*uring.CQEvent) {
	cqes := u.ring.PeekCQEventBatch(buff)

	for i := 0; i < cqes; i++ {
		cqe := buff[i]
		result := cqe.Res
		err := cqe.Error()
		ptr := unsafe.Pointer(uintptr(cqe.UserData))
		cb := *(*callback)(ptr)

		go func() {
			defer func() {
				if r := recover(); r != nil {
				}
			}()
			cb(result, err)
		}()
	}
	u.ring.AdvanceCQ(uint32(cqes))

	buff = buff[:0]
}

func (u *UBatcher) Shutdown() {
	atomic.StoreInt64(&u.shutdown, 1)
}

func (u *UBatcher) Wait() {
	<-u.cqDone
}

func (u *UBatcher) Close() error {
	u.Shutdown()
	u.Wait()
	return u.ring.Close()
}

func (u *UBatcher) Run() {
	go u.CQEventsHandlerRun()
}
