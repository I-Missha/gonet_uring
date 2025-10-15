package ubatcher

import (
	"log"
	"sync"
)

type Buffer struct {
	elements []*Entry
	mut sync.Mutex
}

func NewBuffer(size uint32) *Buffer {
	log.Printf("[Buffer] Создан новый буфер размером %d", size)
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

func (b *Buffer) Size() int {
	b.mut.Lock()
	defer b.mut.Unlock()
	return len(b.elements)
}




