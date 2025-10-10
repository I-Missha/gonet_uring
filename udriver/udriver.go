package ubatcher

import (
	"math/rand/v2"
	"sync"
	"time"

	"github.com/godzie44/go-uring/uring"
)

type callback func()

// inside of uring event resutl we have user data, user data is request id and after getting cqe we can call the corresponding callback
// so, there will be cqe event handler

type UBatcher struct {
	ring *uring.Ring

	callbackMut sync.Mutex
	callbacks   map[uint64]callback // think about this, we add element to map, but we don't know when it will be removed after delete operation
	nextID      uint64

	buffer      *Buffer
	batchSignal chan struct{}
	batchSize   uint32
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

	return &UBatcher{
		ring:        ring,
		callbacks:   make(map[uint64]callback),
		nextID:      rand.Uint64(),
		callbackMut: sync.Mutex{},
		buffer:      NewBuffer(DefaultBufferSize),
		batchSignal: make(chan struct{}, 1),
		batchSize:   DefaultBatchSize,
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

	if len(u.buffer.elements) >= int(u.batchSize) {
		u.batchSignal <- struct{}{}
	}
}

/*
func (u *UBatcher) CQEventsHandlerRun() {
runtime.LockOSThread()

for {
cqe, err := u.ring.WaitCQEvents(1)
if err != nil {
panic(err)
}

u.m.RLock()

u.callbacks[cqe.UserData]()
delete(u.callbacks, cqe.UserData) // another shitty code :D

u.m.RUnlock()
}
}
*/

type Entry struct {
	operation uring.Operation
	cb        callback
}

func handleBatch(u *UBatcher) {
	events := u.buffer.GetAll()
	for _, e := range events {
		u.addToUring(e.operation, e.cb)
	}

	u.ring.Submit()
}
func (u *UBatcher) Run() {
	// runtime.LockOSThread() think about this one

	timer := time.NewTimer(DefaultTimout)
	for {
		select {
		case <-u.batchSignal:
			handleBatch(u)

		case <-timer.C:
			handleBatch(u)
		}
	}

}
