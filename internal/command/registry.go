package command

import (
	"fmt"
	"strings"

	"tiny-redis-go/internal/resp"
)

type Handler func(args []string) resp.Value

type Registry struct {
	handlers map[string]Handler
	order    []string
}

func NewRegistry() *Registry {
	r := &Registry{
		handlers: make(map[string]Handler),
	}

	r.Register("PING", handlePing)
	r.Register("ECHO", handleEcho)
	r.Register("COMMAND", r.handleCommand)

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
