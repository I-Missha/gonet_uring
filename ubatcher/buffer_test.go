package ubatcher

import (
	"sync"
	"testing"
)

func TestBuffer_Put(t *testing.T) {
	buf := NewBuffer(5)
	entry := &Entry{}

	buf.Put(entry)

	if len(buf.elements) != 1 {
		t.Errorf("Expected length 1, got %d", len(buf.elements))
	}

	if buf.elements[0] != entry {
		t.Error("Entry was not stored correctly")
	}
}

func TestBuffer_PutMultiple(t *testing.T) {
	buf := NewBuffer(5)
	entries := []*Entry{{}, {}, {}}

	for _, entry := range entries {
		buf.Put(entry)
	}

	if len(buf.elements) != 3 {
		t.Errorf("Expected length 3, got %d", len(buf.elements))
	}

	for i, entry := range entries {
		if buf.elements[i] != entry {
			t.Errorf("Entry %d was not stored correctly", i)
		}
	}
}

func TestBuffer_GetAll(t *testing.T) {
	buf := NewBuffer(5)
	entries := []*Entry{{}, {}, {}}

	for _, entry := range entries {
		buf.Put(entry)
	}

	retrieved := buf.GetAll()

	if len(retrieved) != 3 {
		t.Errorf("Expected length 3, got %d", len(retrieved))
	}

	for i, entry := range entries {
		if retrieved[i] != entry {
			t.Errorf("Entry %d was not retrieved correctly", i)
		}
	}

	// Проверяем что буфер очистился
	if len(buf.elements) != 0 {
		t.Errorf("Expected buffer to be empty after GetAll, got length %d", len(buf.elements))
	}

	// Проверяем что capacity сохранилась
	if cap(buf.elements) != 5 {
		t.Errorf("Expected capacity to remain 5, got %d", cap(buf.elements))
	}
}

func TestBuffer_GetAllEmpty(t *testing.T) {
	buf := NewBuffer(5)
	retrieved := buf.GetAll()

	if len(retrieved) != 0 {
		t.Errorf("Expected empty slice, got length %d", len(retrieved))
	}
}

func TestBuffer_ConcurrentAccess(t *testing.T) {
	buf := NewBuffer(100)
	var wg sync.WaitGroup

	// Запускаем несколько горутин для записи
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for range 10 {
				buf.Put(&Entry{})
			}
		}()
	}

	wg.Wait()

	retrieved := buf.GetAll()
	if len(retrieved) != 100 {
		t.Errorf("Expected 100 entries, got %d", len(retrieved))
	}
}

