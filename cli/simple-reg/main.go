package main

import (
	"os"

	simpleserver "github.com/nilspolek/simple/internal/server/simple-server"
	"github.com/rs/zerolog"
)

func main() {
	logger := zerolog.New(os.Stdout).
		With().
		Timestamp().
		Logger()

	simpleserver.
		New().
		WithLogRequest().
		WithPort(5000).
		WithLogger(logger).
		ListenAndServe()
}
