package omohan

import (
	"net/http"
)

type Plugin interface {
	InitCraw(client *http.Client) (Plugin, error)
	Baren(string, chan *Info, chan string, int, string) error
}
