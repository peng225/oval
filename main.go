package main

import (
	"log"

	"github.com/peng225/oval/internal/cmd"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	cmd.Execute()
}
