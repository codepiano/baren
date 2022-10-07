package utils

import (
	"io"
)

func StreamToString(stream io.ReadCloser) (string, error) {
	bytes, err := io.ReadAll(stream)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
