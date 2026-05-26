package command

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"tiny-redis-go/internal/resp"
	"tiny-redis-go/internal/store"
)

type Handler func(args []string) resp.Value

type Registry struct {
	handlers map[string]Handler
	order    []string
	store    *store.Store
}

func NewRegistry(db *store.Store) *Registry {
	r := &Registry{
		handlers: make(map[string]Handler),
		store:    db,
	}

	r.Register("PING", r.handlePing)
	r.Register("ECHO", r.handleEcho)
	r.Register("COMMAND", r.handleCommand)
	r.Register("SET", r.handleSet)
	r.Register("GET", r.handleGet)
	r.Register("DEL", r.handleDel)
	r.Register("EXISTS", r.handleExists)
	r.Register("DBSIZE", r.handleDBSize)
	r.Register("EXPIRE", r.handleExpire)
	r.Register("PEXPIRE", r.handlePExpire)
	r.Register("TTL", r.handleTTL)
	r.Register("PTTL", r.handlePTTL)
	r.Register("PERSIST", r.handlePersist)

	return r
}

func (r *Registry) Register(name string, handler Handler) {
	upper := strings.ToUpper(name)
	if _, exists := r.handlers[upper]; !exists {
		r.order = append(r.order, upper)
	}
	r.handlers[upper] = handler
}

func (r *Registry) Dispatch(parts []string) resp.Value {
	if len(parts) == 0 {
		return resp.ErrorString("ERR empty command")
	}

	if r.store != nil {
		r.store.RecordCommand()
	}

	name := strings.ToUpper(parts[0])
	handler, ok := r.handlers[name]
	if !ok {
		return resp.ErrorString(fmt.Sprintf("ERR unknown command '%s'", strings.ToLower(parts[0])))
	}

	return handler(parts[1:])
}

func (r *Registry) handleCommand(_ []string) resp.Value {
	values := make(resp.Array, 0, len(r.order))
	for _, name := range r.order {
		values = append(values, resp.BulkString{Value: name})
	}
	return values
}

func (r *Registry) handlePing(args []string) resp.Value {
	if len(args) == 0 {
		return resp.SimpleString("PONG")
	}
	if len(args) == 1 {
		return resp.BulkString{Value: args[0]}
	}
	return resp.ErrorString("ERR wrong number of arguments for 'ping' command")
}

func (r *Registry) handleEcho(args []string) resp.Value {
	if len(args) != 1 {
		return resp.ErrorString("ERR wrong number of arguments for 'echo' command")
	}
	return resp.BulkString{Value: args[0]}
}

func (r *Registry) handleSet(args []string) resp.Value {
	if len(args) != 2 && len(args) != 4 {
		return resp.ErrorString("ERR wrong number of arguments for 'set' command")
	}
	if err := validateKey(args[0]); err != nil {
		return resp.ErrorString(err.Error())
	}

	if len(args) == 2 {
		r.store.Set(args[0], []byte(args[1]))
		return resp.SimpleString("OK")
	}

	duration, errResp := parseSetExpiration(args[2], args[3])
	if errResp != "" {
		return errResp
	}

	r.store.SetWithTTL(args[0], []byte(args[1]), duration)
	return resp.SimpleString("OK")
}

func (r *Registry) handleGet(args []string) resp.Value {
	if len(args) != 1 {
		return resp.ErrorString("ERR wrong number of arguments for 'get' command")
	}
	if err := validateKey(args[0]); err != nil {
		return resp.ErrorString(err.Error())
	}

	value, ok := r.store.Get(args[0])
	if !ok {
		return resp.BulkString{Null: true}
	}
	return resp.BulkString{Value: string(value.Data)}
}

func (r *Registry) handleDel(args []string) resp.Value {
	if len(args) < 1 {
		return resp.ErrorString("ERR wrong number of arguments for 'del' command")
	}
	for _, key := range args {
		if err := validateKey(key); err != nil {
			return resp.ErrorString(err.Error())
		}
	}

	return resp.Integer(r.store.Del(args...))
}

func (r *Registry) handleExists(args []string) resp.Value {
	if len(args) < 1 {
		return resp.ErrorString("ERR wrong number of arguments for 'exists' command")
	}
	for _, key := range args {
		if err := validateKey(key); err != nil {
			return resp.ErrorString(err.Error())
		}
	}

	return resp.Integer(r.store.Exists(args...))
}

func (r *Registry) handleDBSize(args []string) resp.Value {
	if len(args) != 0 {
		return resp.ErrorString("ERR wrong number of arguments for 'dbsize' command")
	}
	return resp.Integer(r.store.DBSize())
}

func (r *Registry) handleExpire(args []string) resp.Value {
	if len(args) != 2 {
		return resp.ErrorString("ERR wrong number of arguments for 'expire' command")
	}
	if err := validateKey(args[0]); err != nil {
		return resp.ErrorString(err.Error())
	}

	seconds, err := parsePositiveInt64(args[1])
	if err != nil {
		return resp.ErrorString("ERR value is not an integer or out of range")
	}

	return resp.Integer(r.store.Expire(args[0], time.Duration(seconds)*time.Second))
}

func (r *Registry) handlePExpire(args []string) resp.Value {
	if len(args) != 2 {
		return resp.ErrorString("ERR wrong number of arguments for 'pexpire' command")
	}
	if err := validateKey(args[0]); err != nil {
		return resp.ErrorString(err.Error())
	}

	milliseconds, err := parsePositiveInt64(args[1])
	if err != nil {
		return resp.ErrorString("ERR value is not an integer or out of range")
	}

	return resp.Integer(r.store.Expire(args[0], time.Duration(milliseconds)*time.Millisecond))
}

func (r *Registry) handleTTL(args []string) resp.Value {
	if len(args) != 1 {
		return resp.ErrorString("ERR wrong number of arguments for 'ttl' command")
	}
	if err := validateKey(args[0]); err != nil {
		return resp.ErrorString(err.Error())
	}

	return resp.Integer(r.store.TTL(args[0]))
}

func (r *Registry) handlePTTL(args []string) resp.Value {
	if len(args) != 1 {
		return resp.ErrorString("ERR wrong number of arguments for 'pttl' command")
	}
	if err := validateKey(args[0]); err != nil {
		return resp.ErrorString(err.Error())
	}

	return resp.Integer(r.store.PTTL(args[0]))
}

func (r *Registry) handlePersist(args []string) resp.Value {
	if len(args) != 1 {
		return resp.ErrorString("ERR wrong number of arguments for 'persist' command")
	}
	if err := validateKey(args[0]); err != nil {
		return resp.ErrorString(err.Error())
	}

	return resp.Integer(r.store.Persist(args[0]))
}

func validateKey(key string) error {
	if key == "" {
		return fmt.Errorf("ERR empty key")
	}
	return nil
}

func parseSetExpiration(option string, rawValue string) (time.Duration, resp.ErrorString) {
	value, err := parsePositiveInt64(rawValue)
	if err != nil {
		return 0, resp.ErrorString("ERR value is not an integer or out of range")
	}
	if value <= 0 {
		return 0, resp.ErrorString("ERR invalid expire time in 'set' command")
	}

	switch strings.ToUpper(option) {
	case "EX":
		return time.Duration(value) * time.Second, ""
	case "PX":
		return time.Duration(value) * time.Millisecond, ""
	default:
		return 0, resp.ErrorString("ERR syntax error")
	}
}

func parsePositiveInt64(raw string) (int64, error) {
	return strconv.ParseInt(raw, 10, 64)
}
