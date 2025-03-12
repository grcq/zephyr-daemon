package main

import (
	"daemon/cmd"
	"github.com/apex/log"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.WithError(err).Fatal("failed to execute command")
	}
}
