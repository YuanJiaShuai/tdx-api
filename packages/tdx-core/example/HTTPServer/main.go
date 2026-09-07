package main

import (
	"github.com/injoyai/logs"
	"github.com/injoyai/tdx/extend/httpserver"
)

func main() {
	s, err := httpserver.Default()
	logs.PanicErr(err)
	s.Run()
}
