# CRUSH - Go io_uring Network Library

## Project Structure
```
/home/obryvko-m/dev/gonet_uring/
├── CRUSH.md
├── go.mod
├── go.sum
├── examples/
│   └── echo_server.go
├── net/
│   ├── dial.go      (DialContext implementation)
│   └── net.go       (UringConn implementation)
└── ubatcher/
    ├── buffer.go        (Buffer for batching operations)
    ├── buffer_test.go
    └── ubatcher.go      (UBatcher - main io_uring wrapper)
```

## Build/Test/Lint Commands
```bash
# Build project
go build ./...

# Run specific example
go run examples/echo_server.go

# Test all packages
go test ./...

# Test specific package
go test ./ubatcher
go test ./net

# Format code
gofmt -w .

# Vet code
go vet ./...

# Mod tidy
go mod tidy
```

## Code Style Guidelines

**Imports:** Group standard library, third-party, and local imports separately. Use named imports for clarity (e.g., `gonet "net"`).

**Naming:** Use camelCase for unexported, PascalCase for exported. Package names are lowercase. Type names should be descriptive (e.g., `UringConn`, `UBatcher`).

**Error Handling:** Return errors as last parameter. Use `errors.New()` for simple errors, define package-level error variables for common errors (e.g., `ErrTimeout`).

**Comments:** Use Russian comments for complex logic, English for exported functions. No unnecessary comments on simple code.

**Types:** Prefer struct embedding over inheritance. Use pointer receivers for methods that modify state or large structs.

**Concurrency:** Use mutexes for critical sections (`sync.Mutex`). Call `runtime.LockOSThread()` for goroutines managing OS resources.

**Memory:** Reuse buffers where possible. Use `make()` with capacity for known sizes.

**Constants:** Group related constants in blocks with descriptive names and comments about defaults.

## О проекте

Конечная цель проекта - создать интерфейс `net.Conn` и метод `DialContext` на основе io_uring для батчинга системных вызовов.

на текущий момент net не реализован

### Пакет ubatcher/
**UBatcher** - основная обертка над io_uring:
- `NewUBatcher()` - создание экземпляра
- `PushOperation()` - добавление операции в буфер
- `CQEventsHandlerRun()` - обработка событий завершения (CQE)
- `Run()` - основной цикл батчинга операций
- `Shutdown()`, `Wait()`, `Close()` - управление жизненным циклом

**Buffer** - буфер для операций перед батчингом:
- `Put()` - добавление операции
- `GetAll()` - получение всех операций с очисткой буфера
- Thread-safe через mutex

### Пакет net/
**UringConn** - реализация `net.Conn` через io_uring:
- `New()` - создание соединения
- `Read()`, `Write()` - основные операции ввода-вывода
- `Close()`, `LocalAddr()`, `RemoteAddr()` - стандартные методы net.Conn
- `SetDeadline()`, `SetReadDeadline()`, `SetWriteDeadline()` - управление таймаутами

**DialContext** - создание соединений через io_uring (в разработке)

### Зависимости
- `github.com/godzie44/go-uring/uring` - обертка над системными вызовами io_uring
- я сделал свой форк от `github.com/godzie44/go-uring`, сейчас используется он, находится в ~/dev/go-uring
- Go 1.23.9

### Константы
- `DefaultBatchSize = 16` - размер батча по умолчанию
- `DefaultBufferSize = 26` - размер буфера (BatchSize + 10 для избежания блокировок)
- `DefaultTimeout = 10ms` - таймаут для принудительной отправки батча
