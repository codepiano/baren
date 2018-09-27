package omohan

import (
	"net/http"
)

type Login interface {
	Login(client *http.Client, config map[string]string)
}
