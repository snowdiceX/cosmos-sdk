package main

import (
	"github.com/QOSGroup/cassini/log"
)

func main() {
	defer log.Flush()

	root := NewRootCommand()
	root.AddCommand(
		NewVersionCommand())

	if err := root.Execute(); err != nil {
		log.Error("Exit by error: ", err)
	}
}
