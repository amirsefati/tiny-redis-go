# tiny-redis-go

A Redis-inspired in-memory database written from scratch in Go.

This project is not intended to be a production-ready Redis replacement. The goal is to deeply understand how Redis-like systems work internally:

- RESP protocol parsing
- TCP server design
- In-memory key-value storage
- TTL and expiration
- Memory-aware data structures
- Append-only persistence
- Eviction policies
- Pub/Sub
- Basic replication concepts
- Observability and benchmarking

Built as a 10-day engineering challenge.

## Day 2 Scope

Day 2 adds a real in-memory database on top of the Day 1 protocol server:

- TCP server on `0.0.0.0:6379`
- Concurrent connection handling with goroutines
- RESP array and bulk string parsing
- RESP simple string, error, integer, bulk string, null bulk string, and array encoding
- Thread-safe in-memory store using `sync.RWMutex`
- Store-backed commands for `SET`, `GET`, `DEL`, `EXISTS`, and `DBSIZE`
- Value metadata with type, timestamps, and versioning
- Defensive memory copying on read and write paths
- Unit tests and benchmarks for the store layer

## Project Structure

```text
tiny-redis-go/
  cmd/
    server/
      main.go
  internal/
    command/
      registry.go
    resp/
      reader.go
      reader_test.go
      types.go
      writer.go
      writer_test.go
    server/
      tcp.go
    store/
      store.go
      store_benchmark_test.go
      store_test.go
  go.mod
  README.md
```

## Run the Server

```bash
go run ./cmd/server
```

## Manual Testing

Using `redis-cli`:

```bash
redis-cli -p 6379 PING
redis-cli -p 6379 ECHO hello
redis-cli -p 6379 COMMAND
redis-cli -p 6379 SET language go
redis-cli -p 6379 GET language
redis-cli -p 6379 EXISTS language missing
redis-cli -p 6379 DEL language
redis-cli -p 6379 DBSIZE
```

Using raw RESP over `nc`:

```bash
printf '*1\r\n$4\r\nPING\r\n' | nc localhost 6379
printf '*2\r\n$4\r\nECHO\r\n$5\r\nhello\r\n' | nc localhost 6379
printf '*3\r\n$3\r\nSET\r\n$8\r\nlanguage\r\n$2\r\ngo\r\n' | nc localhost 6379
printf '*2\r\n$3\r\nGET\r\n$8\r\nlanguage\r\n' | nc localhost 6379
```

## Test

```bash
go test ./...
```

## Benchmark

```bash
go test -bench=. ./internal/store
```

## Design Notes

### Why Redis uses a custom protocol

Redis is optimized for a tight request-response loop. A custom protocol gives it:

- predictable framing over TCP
- low parsing overhead
- language-agnostic client implementations
- direct mapping between command arguments and wire format

For a memory-first database, protocol simplicity matters because every extra branch in the hot path adds latency and complexity.

### Why RESP is simple and efficient

RESP is cheap to parse because it is explicit and deterministic:

- each value starts with a type prefix
- bulk strings are length-prefixed, so payloads do not need escaping
- arrays make command framing unambiguous

That combination makes it practical to implement in a few hundred lines while still being robust enough for high-throughput systems.

### Goroutine-per-connection vs Redis event loop

This implementation uses one goroutine per connection because it matches Go's concurrency model well:

- the code stays straightforward
- the standard library handles most scheduling concerns
- connection isolation is easy to reason about

Real Redis historically uses a single-threaded event loop for command execution because it avoids lock contention and keeps memory access patterns extremely predictable. Go's goroutine approach is a good educational tradeoff for Day 1, but it is not a perfect mirror of Redis internals.

### Why `map[string][]byte` is fast but limited

Go maps are a strong starting point because lookup and update are efficient and the implementation is battle-tested. Storing raw bytes also keeps the interface flexible:

- strings can be returned without serialization overhead beyond RESP framing
- future encodings can reuse the same binary container
- the store can evolve beyond plain text values

The limitation is that a bare map is not yet a Redis-like object system. Real Redis values carry type information, encoding choices, metadata, and lifecycle rules. That is why this project already wraps the raw bytes in a `Value` struct instead of exposing `map[string][]byte` directly.

### Why memory ownership matters

In an in-memory database, ownership bugs are data corruption bugs. If the server stored references to a parser buffer directly:

- the parser could reuse that buffer for the next command
- another goroutine could observe mutated contents
- callers could accidentally change stored values after insertion

This implementation copies bytes on `SET` and returns copies on `GET`. That costs allocations, but it gives the store a clear ownership boundary.

### Why copying is safer but more expensive

Copying data protects correctness:

- stored values cannot be mutated by request handlers after insertion
- returned values cannot mutate the database accidentally
- future refactors are less likely to introduce aliasing bugs

The downside is more allocation and memory bandwidth use. That tradeoff is fine for Day 2 because correctness and clarity matter more than micro-optimizing the hot path too early.

### How Redis stores values internally

Redis does not treat values as plain strings in the general case. It uses internal objects and encodings so the same logical type can have different physical layouts depending on size and usage patterns. A small integer-like string may be encoded very differently from a larger heap-allocated string. That object model is one reason Redis can stay memory-efficient while supporting many data types.

This Go version is only taking the first step in that direction:

- a `Type` field for future data structures
- raw byte payloads for current string values
- metadata for timestamps and versioning

### Current limitations

This server is still intentionally small. It does not yet include:

- pipelining optimizations
- transactions
- persistence
- replication
- eviction
- authentication
- graceful in-flight connection draining on shutdown

It is a protocol server first. Storage comes next.
