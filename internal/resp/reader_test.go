package resp

import (
	"errors"
	"strings"
	"testing"
)

func TestReadCommand(t *testing.T) {
	t.Parallel()

	reader := NewReader(strings.NewReader("*2\r\n$4\r\nECHO\r\n$5\r\nhello\r\n"))

	command, err := reader.ReadCommand()
	if err != nil {
		t.Fatalf("ReadCommand() error = %v", err)
	}

	expected := []string{"ECHO", "hello"}
	if len(command) != len(expected) {
		t.Fatalf("unexpected command length: got %d want %d", len(command), len(expected))
	}

	for i := range expected {
		if command[i] != expected[i] {
			t.Fatalf("unexpected command[%d]: got %q want %q", i, command[i], expected[i])
		}
	}
}

func TestReadCommandProtocolError(t *testing.T) {
	t.Parallel()

	reader := NewReader(strings.NewReader("+PING\r\n"))

	_, err := reader.ReadCommand()
	if err == nil {
		t.Fatal("expected protocol error, got nil")
	}

	var protocolErr *ProtocolError
	if !errors.As(err, &protocolErr) {
		t.Fatalf("expected ProtocolError, got %T", err)
	}
}

func TestReadBulkStringMissingCRLF(t *testing.T) {
	t.Parallel()

	reader := NewReader(strings.NewReader("$5\r\nhelloX"))

	_, err := reader.ReadValue()
	if err == nil {
		t.Fatal("expected protocol error, got nil")
	}

	var protocolErr *ProtocolError
	if !errors.As(err, &protocolErr) {
		t.Fatalf("expected ProtocolError, got %T", err)
	}
}
