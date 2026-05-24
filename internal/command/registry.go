package command

import (
	"fmt"
	"strings"

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
	if len(args) != 2 {
		return resp.ErrorString("ERR wrong number of arguments for 'set' command")
	}
	if err := validateKey(args[0]); err != nil {
		return resp.ErrorString(err.Error())
	}

	r.store.Set(args[0], []byte(args[1]))
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

func validateKey(key string) error {
	if key == "" {
		return fmt.Errorf("ERR empty key")
	}
	return nil
}
