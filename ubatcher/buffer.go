package ubatcher

import (
	"sync"

	"github.com/godzie44/go-uring/uring"
)

type Entry struct {
	operation uring.Operation
	cb        callback
}

var entryPool = sync.Pool{
	New: func() interface{} {
		return &Entry{}
	},
}

const defaultBufferSize = 128
var slicePool = sync.Pool{
	New: func() interface{} {
		return make([]*Entry, 0, defaultBufferSize)
	},
}

var cqeBufferPool = sync.Pool{
	New: func() interface{} {
		return make([]*uring.CQEvent, CQEventsToWait * 2)
	},
}

func Init() {
	// Предзаполняем пулы для уменьшения аллокаций в hot path
	for i := 0; i < 1000; i++ {
		entryPool.Put(&Entry{})
	}

	for i := 0; i < 50; i++ {
		slicePool.Put(make([]*Entry, 0, 64))
	}

	for i := 0; i < 10; i++ {
		cqeBufferPool.Put(make([]*uring.CQEvent, 20))
	}
}

func acquireEntry() *Entry {
	return entryPool.Get().(*Entry)
}

func releaseEntry(e *Entry) {
	e.operation = nil
	e.cb = nil
	entryPool.Put(e)
}

func acquireSlice() []*Entry {
	s := slicePool.Get().([]*Entry)
	return s[:0]
}

func releaseSlice(s []*Entry) {
	slicePool.Put(s)
}

func acquireCQEBuffer() []*uring.CQEvent {
	return cqeBufferPool.Get().([]*uring.CQEvent)
}

func releaseCQEBuffer(buf []*uring.CQEvent) {
	cqeBufferPool.Put(buf)
}

type Buffer struct {
	elements []*Entry
	mut      sync.Mutex
}

func NewBuffer(size uint32) *Buffer {
	return &Buffer{
		elements: acquireSlice(),
		mut:      sync.Mutex{},
	}
}

func (b *Buffer) Put(e *Entry) {
	b.mut.Lock()
	defer b.mut.Unlock()

	b.elements = append(b.elements, e)
}

func (b *Buffer) GetAll() []*Entry {
	b.mut.Lock()
	defer b.mut.Unlock()

	toRet := make([]*Entry, len(b.elements))
	copy(toRet, b.elements)
	b.elements = b.elements[:0]
	return toRet
}

func (b *Buffer) Size() int {
	b.mut.Lock()
	defer b.mut.Unlock()
	return len(b.elements)
}




