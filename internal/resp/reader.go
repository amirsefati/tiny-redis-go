package resp

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Reader decodes RESP values from a buffered stream.
type Reader struct {
	reader *bufio.Reader
}

func NewReader(r io.Reader) *Reader {
	return &Reader{reader: bufio.NewReader(r)}
}

func (r *Reader) ReadValue() (Value, error) {
	prefix, err := r.reader.ReadByte()
	if err != nil {
		return nil, err
	}

	switch prefix {
	case '*':
		return r.readArray()
	case '$':
		return r.readBulkString()
	default:
		return nil, &ProtocolError{Message: fmt.Sprintf("unexpected prefix %q", prefix)}
	}
}

func (r *Reader) ReadCommand() ([]string, error) {
	value, err := r.ReadValue()
	if err != nil {
		return nil, err
	}

	array, ok := value.(Array)
	if !ok {
		return nil, &ProtocolError{Message: "expected array command"}
	}

	command := make([]string, 0, len(array))
	for _, item := range array {
		bulk, ok := item.(BulkString)
		if !ok || bulk.Null {
			return nil, &ProtocolError{Message: "expected non-null bulk string in command"}
		}
		command = append(command, bulk.Value)
	}

	return command, nil
}

func (r *Reader) readArray() (Value, error) {
	count, err := r.readLengthLine()
	if err != nil {
		return nil, err
	}
	if count < 0 {
		return nil, &ProtocolError{Message: "array length cannot be negative"}
	}

	values := make(Array, 0, count)
	for range count {
		value, err := r.ReadValue()
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}

	return values, nil
}

func (r *Reader) readBulkString() (Value, error) {
	length, err := r.readLengthLine()
	if err != nil {
		return nil, err
	}

	if length == -1 {
		return BulkString{Null: true}, nil
	}
	if length < -1 {
		return nil, &ProtocolError{Message: "invalid bulk string length"}
	}

	data := make([]byte, length+2)
	if _, err := io.ReadFull(r.reader, data); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil, &ProtocolError{Message: "incomplete bulk string payload"}
		}
		return nil, err
	}
	if data[length] != '\r' || data[length+1] != '\n' {
		return nil, &ProtocolError{Message: "bulk string missing CRLF terminator"}
	}

	return BulkString{Value: string(data[:length])}, nil
}

func (r *Reader) readLengthLine() (int, error) {
	line, err := r.readLine()
	if err != nil {
		return 0, err
	}

	value, err := strconv.Atoi(line)
	if err != nil {
		return 0, &ProtocolError{Message: fmt.Sprintf("invalid length %q", line)}
	}

	return value, nil
}

func (r *Reader) readLine() (string, error) {
	line, err := r.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	if !strings.HasSuffix(line, "\r\n") {
		return "", &ProtocolError{Message: "line does not end with CRLF"}
	}

	return strings.TrimSuffix(line, "\r\n"), nil
}
