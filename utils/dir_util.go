package utils

import (
	"os"
)

func MkdirIfNotExist(path string, mode os.FileMode) error {
	// 判断目录是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, mode)
		return err
	}
	return nil
}
