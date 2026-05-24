package resp

// Value represents a RESP value supported by this Day 1 implementation.
type Value interface{}

// SimpleString encodes RESP simple string values like +OK.
type SimpleString string

// ErrorString encodes RESP error values like -ERR message.
type ErrorString string

// Integer encodes RESP integer values like :1.
type Integer int64

// BulkString encodes RESP bulk string values.
type BulkString struct {
	Value string
	Null  bool
}

// Array encodes RESP arrays.
type Array []Value

// ProtocolError is returned when the incoming RESP payload is malformed.
type ProtocolError struct {
	Message string
}

func (e *ProtocolError) Error() string {
	return "protocol error: " + e.Message
}
