package main

import "io"

type Plugin interface {
	Baren(io.Reader, func(string) io.ReadCloser, chan []string)
}
