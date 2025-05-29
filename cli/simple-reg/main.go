package main

import (
	"flag"
	"os"

	simpleserver "github.com/nilspolek/simple-reg/internal/server/simple-server"
	"github.com/rs/zerolog"
)

var (
	port      string
	isVerbose bool
)

func main() {
	flag.StringVar(&port, "port", "5000", "port to listen on")
	flag.BoolVar(&isVerbose, "verbose", false, "verbose logging")
	flag.Parse()

	logger := zerolog.New(os.Stdout).
		With().
		Timestamp().
		Logger().
		Level(zerolog.ErrorLevel)

	if isVerbose {
		logger = logger.Level(zerolog.DebugLevel)
	}

	simpleserver.
		New().
		WithLogRequest().
		WithPort(5000).
		WithLogger(logger).
		ListenAndServe()
}
