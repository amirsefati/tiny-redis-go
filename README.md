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

## Day 3 Scope

Day 3 adds TTL-aware storage and Redis-style expiration behavior on top of the Day 2 store:

- TCP server on `0.0.0.0:6379`
- Concurrent connection handling with goroutines
- RESP array and bulk string parsing
- RESP simple string, error, integer, bulk string, null bulk string, and array encoding
- Thread-safe in-memory store using `sync.RWMutex`
- Store-backed commands for `SET`, `GET`, `DEL`, `EXISTS`, `DBSIZE`, `EXPIRE`, `PEXPIRE`, `TTL`, `PTTL`, and `PERSIST`
- `SET key value EX seconds` and `SET key value PX milliseconds`
- Value metadata with type, timestamps, versioning, and optional expiration
- Lazy expiration on read-oriented commands
- Active expiration with a background cleanup goroutine
- Configurable sampled cleanup instead of full keyspace scans
- Expiration statistics for deleted stale keys and cleanup cycles
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
redis-cli -p 6379 SET session abc EX 10
redis-cli -p 6379 TTL session
redis-cli -p 6379 PTTL session
redis-cli -p 6379 EXPIRE language 30
redis-cli -p 6379 PERSIST language
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
printf '*5\r\n$3\r\nSET\r\n$7\r\nsession\r\n$3\r\nabc\r\n$2\r\nPX\r\n$4\r\n1500\r\n' | nc localhost 6379
printf '*2\r\n$4\r\nPTTL\r\n$7\r\nsession\r\n' | nc localhost 6379
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

### Why Redis uses both lazy and active expiration

Expiration is not just a timestamp feature. It is a memory-management strategy.

Lazy expiration keeps the hot path cheap. When a client touches a key through `GET`, `EXISTS`, `TTL`, or `PTTL`, the store checks the key's expiration and deletes it on demand if it is stale. That means keys that are never touched again do not cost work on every tick.

Active expiration solves the opposite problem. If the database only used lazy deletion, expired keys that nobody reads anymore would stay in memory forever. That turns TTL into a logical visibility rule but not a physical cleanup rule. The background worker fixes that by periodically sampling a subset of keys and deleting stale ones.

Using both strategies gives a practical balance:

- lazy deletion keeps normal reads simple and accurate
- active cleanup prevents dead keys from accumulating in memory
- sampling limits cleanup cost so latency stays predictable

### Why scanning the entire keyspace is dangerous

A full scan on every cleanup tick sounds simple, but it scales poorly:

- work grows linearly with database size
- a large keyspace can monopolize the CPU
- long cleanup pauses can delay foreground requests
- lock hold times get worse under concurrency

This implementation follows the same broad idea Redis uses: sample a bounded number of keys per cleanup cycle instead of walking the entire map every time. That makes cleanup cost configurable and keeps the tail latency story much healthier.

### How TTL metadata affects memory usage

Every key now carries an extra `ExpiresAt` field. That makes each value slightly larger, and it also means the store may temporarily hold logically expired keys until lazy or active cleanup removes them.

That overhead is the tradeoff for fast expiration decisions:

- reads can decide in constant time whether a key is stale
- cleanup can delete without consulting another index
- semantics stay local to the value object

Even in a simple implementation, TTL is already shaping memory layout and lifecycle behavior, not just command syntax.

### Why time handling is subtle

Expiration code looks simple until time semantics get involved.

This project stores expiration as a Unix millisecond timestamp because Redis exposes both second and millisecond TTL commands. That keeps command behavior straightforward, but there are still subtleties:

- `TTL` and `PTTL` need different units
- rounding can make `TTL` return `0` for a key that still has a few hundred milliseconds left
- wall-clock time can move unexpectedly if the system clock changes
- elapsed-time logic is often safer with monotonic clocks than with raw wall-clock timestamps

Go's `time.Time` carries a monotonic component, but `UnixMilli()` does not. For a learning project, Unix milliseconds are a clear fit for the Redis API. For more production-like behavior, it is worth thinking carefully about how internal elapsed-time measurement and external timestamp reporting should interact.

### Locking strategy and lock-upgrade pitfalls

The store uses a two-phase approach for lazy expiration:

- read the key under `RLock`
- if it looks expired, drop the read lock
- reacquire a full `Lock`
- re-check expiration and delete only if it is still stale

That matters because `sync.RWMutex` does not support lock upgrade. Trying to delete while still holding a read lock would either deadlock or force unsafe patterns. Re-checking after acquiring the write lock avoids deleting a key that another goroutine may have refreshed between the read and write phases.

### How expiration will interact with persistence and replication later

TTL has downstream effects beyond memory cleanup.

For persistence:

- snapshots need to decide whether to write absolute expiry times or remaining TTL
- append-only logs need to record expiration-changing commands consistently
- loading data back must preserve correct expiration semantics

For replication:

- replicas need deterministic expiration behavior
- masters often propagate expiration as explicit deletes or expiration commands
- clock skew becomes a real design concern

That is why expiration is an important systems feature, not just an extra field on a struct.

## Day 3 Post Angle

Suggested title:

**Building Redis from Scratch in Go — Day 3: TTL, Expiration, and Lazy Deletion**

Professional framing:

- expiration is not just a timestamp; it is a memory-management strategy
- without active cleanup, expired keys can remain in memory indefinitely
- too much cleanup hurts latency
- sampling and heuristics are how Redis navigates that tradeoff

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
