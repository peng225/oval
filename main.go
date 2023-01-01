package main

import (
	"log"

	"github.com/peng225/oval/cmd"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	cmd.Execute()
}
