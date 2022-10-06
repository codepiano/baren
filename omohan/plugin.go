package omohan

import (
	"io"
	"net/http"
)

type Plugin interface {
	InitCraw(client *http.Client) *Plugin
	Baren(string, func(string) io.ReadCloser, chan *Info, chan string, int, string)
}
