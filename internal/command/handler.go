package command

import "tiny-redis-go/internal/resp"

func handlePing(args []string) resp.Value {
	if len(args) == 0 {
		return resp.SimpleString("PONG")
	}
	if len(args) == 1 {
		return resp.BulkString{Value: args[0]}
	}
	return resp.ErrorString("ERR wrong number of arguments for 'ping' command")
}

func handleEcho(args []string) resp.Value {
	if len(args) != 1 {
		return resp.ErrorString("ERR wrong number of arguments for 'echo' command")
	}
	return resp.BulkString{Value: args[0]}
}
