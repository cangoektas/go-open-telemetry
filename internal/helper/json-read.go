package helper

import (
	"encoding/json"
	"io"
)

func JsonRead[T any](r io.ReadCloser, t T) error {
	str, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	err = json.Unmarshal(str, &t)
	if err != nil {
		return err
	}

	return nil
}
