package omohan

import (
	"io"
)

type Plugin interface {
	Baren(string, func(string) io.ReadCloser, chan *Info, chan string)
}
