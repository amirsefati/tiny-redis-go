package resp

import (
	"bytes"
	"testing"
)

func TestWriteValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value Value
		want  string
	}{
		{
			name:  "simple string",
			value: SimpleString("PONG"),
			want:  "+PONG\r\n",
		},
		{
			name:  "error string",
			value: ErrorString("ERR failure"),
			want:  "-ERR failure\r\n",
		},
		{
			name:  "integer",
			value: Integer(2),
			want:  ":2\r\n",
		},
		{
			name:  "bulk string",
			value: BulkString{Value: "hello"},
			want:  "$5\r\nhello\r\n",
		},
		{
			name:  "null bulk string",
			value: BulkString{Null: true},
			want:  "$-1\r\n",
		},
		{
			name: "array",
			value: Array{
				BulkString{Value: "PING"},
				BulkString{Value: "ECHO"},
			},
			want: "*2\r\n$4\r\nPING\r\n$4\r\nECHO\r\n",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buffer bytes.Buffer
			writer := NewWriter(&buffer)

			if err := writer.WriteValue(tt.value); err != nil {
				t.Fatalf("WriteValue() error = %v", err)
			}

			if got := buffer.String(); got != tt.want {
				t.Fatalf("unexpected output: got %q want %q", got, tt.want)
			}
		})
	}
}
