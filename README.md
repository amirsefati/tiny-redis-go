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

## Day 1 Scope

Day 1 focuses on the network and protocol boundary:

- TCP server on `0.0.0.0:6379`
- Concurrent connection handling with goroutines
- RESP array and bulk string parsing
- RESP simple string, error, bulk string, null bulk string, and array encoding
- Command dispatch for `PING`, `ECHO`, and `COMMAND`
- Unit tests for RESP parsing and writing

## Project Structure

```text
tiny-redis-go/
  cmd/
    server/
      main.go
  internal/
    command/
      handler.go
      registry.go
    resp/
      reader.go
      reader_test.go
      types.go
      writer.go
      writer_test.go
    server/
      tcp.go
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
```

Using raw RESP over `nc`:

```bash
printf '*1\r\n$4\r\nPING\r\n' | nc localhost 6379
printf '*2\r\n$4\r\nECHO\r\n$5\r\nhello\r\n' | nc localhost 6379
```

## Test

```bash
go test ./...
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

### Day 1 limitations

This server is intentionally small. It does not yet include:

- key-value storage
- pipelining optimizations
- transactions
- persistence
- replication
- eviction
- authentication
- graceful in-flight connection draining on shutdown

It is a protocol server first. Storage comes next.
