package ubatcher

import (
	"sync"
)

type Buffer struct {
	elements []*Entry
	mut sync.Mutex
}

func NewBuffer(size uint32) *Buffer {
	return &Buffer{
		elements: make([]*Entry, 0, size),
		mut: sync.Mutex{},
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

	toRet := b.elements
	b.elements = make([]*Entry, 0, cap(b.elements))
	return toRet
}




