package server

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"strings"
	"sync"

	"tiny-redis-go/internal/command"
	"tiny-redis-go/internal/resp"
)

type Server struct {
	addr     string
	registry *command.Registry
	logger   *log.Logger
}

func New(addr string, registry *command.Registry, logger *log.Logger) *Server {
	return &Server{
		addr:     addr,
		registry: registry,
		logger:   logger,
	}
}

func (s *Server) Addr() string {
	return s.addr
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	var once sync.Once
	go func() {
		<-ctx.Done()
		once.Do(func() {
			_ = listener.Close()
		})
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Temporary() {
				s.logger.Printf("temporary accept error: %v", err)
				continue
			}
			return err
		}

		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := resp.NewReader(conn)
	writer := resp.NewWriter(conn)

	for {
		commandParts, err := reader.ReadCommand()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}

			var protocolErr *resp.ProtocolError
			if errors.As(err, &protocolErr) {
				if writeErr := writer.WriteValue(resp.ErrorString("ERR " + protocolErr.Error())); writeErr != nil {
					s.logger.Printf("failed to write protocol error response: %v", writeErr)
				}
				return
			}

			if isConnectionClosed(err) {
				return
			}

			s.logger.Printf("connection read error: %v", err)
			return
		}

		response := s.registry.Dispatch(commandParts)
		if err := writer.WriteValue(response); err != nil {
			if isConnectionClosed(err) {
				return
			}
			s.logger.Printf("connection write error: %v", err)
			return
		}
	}
}

func isConnectionClosed(err error) bool {
	if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "broken pipe") ||
		strings.Contains(strings.ToLower(err.Error()), "connection reset by peer")
}
