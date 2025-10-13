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
	log.Printf("[Buffer] Добавлена операция в буфер, всего элементов: %d", len(b.elements))
}

func (b *Buffer) GetAll() []*Entry {
	b.mut.Lock()
	defer b.mut.Unlock()

	toRet := b.elements
	count := len(toRet)
	b.elements = make([]*Entry, 0, cap(b.elements))
	log.Printf("[Buffer] Извлечено %d операций из буфера", count)
	return toRet
}




