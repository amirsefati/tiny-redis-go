package resp

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

// Writer encodes RESP values to a stream.
type Writer struct {
	writer *bufio.Writer
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{writer: bufio.NewWriter(w)}
}

func (w *Writer) WriteValue(value Value) error {
	if err := w.writeValue(value); err != nil {
		return err
	}
	return w.writer.Flush()
}

func (w *Writer) writeValue(value Value) error {
	switch v := value.(type) {
	case SimpleString:
		_, err := fmt.Fprintf(w.writer, "+%s\r\n", string(v))
		return err
	case ErrorString:
		_, err := fmt.Fprintf(w.writer, "-%s\r\n", string(v))
		return err
	case Integer:
		_, err := fmt.Fprintf(w.writer, ":%d\r\n", int64(v))
		return err
	case BulkString:
		if v.Null {
			_, err := w.writer.WriteString("$-1\r\n")
			return err
		}
		if _, err := w.writer.WriteString("$" + strconv.Itoa(len(v.Value)) + "\r\n"); err != nil {
			return err
		}
		if _, err := w.writer.WriteString(v.Value); err != nil {
			return err
		}
		_, err := w.writer.WriteString("\r\n")
		return err
	case Array:
		if _, err := w.writer.WriteString("*" + strconv.Itoa(len(v)) + "\r\n"); err != nil {
			return err
		}
		for _, item := range v {
			if err := w.writeValue(item); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported RESP value type %T", value)
	}
}
