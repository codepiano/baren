package utils

import (
	"strings"
)

func RemoveLeftMostToEnd(s string, r rune) string {
	i := strings.Index(s, string(r))
	if i > -1 {
		return s[:i]
	} else {
		return s
	}
}

func RemoveStartToRightMost(s string, r rune) string {
	i := strings.LastIndex(s, string(r))
	if i > -1 {
		return s[i+1:]
	} else {
		return s
	}
}

func NormalizePath(path string) string {
	path = strings.Replace(path, "\\/", "_", -1)
	path = strings.Replace(path, "/\\", "_", -1)
	path = strings.Replace(path, "/", "_", -1)
	return path
}
