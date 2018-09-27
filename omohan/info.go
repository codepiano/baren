package omohan

import (
	"fmt"
	"net/http"
)

type Info struct {
	Request  *http.Request
	FileName string
	Dir      string
}

func (info Info) String() string {
	return fmt.Sprintf("%s, %s", info.Request.URL.String(), info.FileName)
}
