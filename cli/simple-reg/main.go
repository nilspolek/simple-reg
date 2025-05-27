package main

import (
	"os"

	blobservice "github.com/nilspolek/simple/internal/blob-service"
	simpleserver "github.com/nilspolek/simple/internal/server/simple-server"
	"github.com/rs/zerolog"
)

func main() {
	logger := zerolog.New(os.Stdout).
		With().
		Timestamp().
		Logger()

	bs := blobservice.New()

	simpleserver.
		New(bs).
		WithLogRequest().
		WithPort(5000).
		WithLogger(logger).
		ListenAndServe()
}
