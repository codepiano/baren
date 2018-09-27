package utils

import (
	"io"
	"io/ioutil"
)

func StreamToString(stream io.ReadCloser) (string, error) {
	bytes, err := ioutil.ReadAll(stream)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
