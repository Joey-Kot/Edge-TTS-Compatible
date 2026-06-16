package sse

import (
	"encoding/json"
	"fmt"
	"io"
)

func Data(w io.Writer, data any) error {
	switch v := data.(type) {
	case string:
		_, err := fmt.Fprintf(w, "data: %s\n\n", v)
		return err
	default:
		encoded, err := json.Marshal(data)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "data: %s\n\n", encoded)
		return err
	}
}
