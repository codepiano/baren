package utils

import (
	"os"
	"regexp"
	"strings"
)

func MkdirIfNotExist(path string, mode os.FileMode) error {
	// 判断目录是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, mode)
		return err
	}
	return nil
}

// CleanLabelText 清理字符串中的下列字符
/*
   ' #xxxx '
   ' #xxx#xxx '
*/
var labelRegex = regexp.MustCompile(`(#\S+)+ ?`)

func CleanLabelText(text string) string {
	return strings.TrimSpace(labelRegex.ReplaceAllString(text, ""))
}

var illegalFileNameChars = regexp.MustCompile(`[/\\:*?"<>|]`)

func RemoveIllegalFileNameChars(fileName string) string {
	return illegalFileNameChars.ReplaceAllString(fileName, "_")
}
