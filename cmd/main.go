package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/OleksandrKuleshov/home-task/internal/app"
)

var (
	storage        = flag.String("storage", "sqlite", "storage engine to use")
	filePath       = flag.String("file", "./ports.json", "path to ports JSON file")
	batchSize      = flag.Int("batch", 100, "batch size for ports to process")
	chanBufferSize = flag.Int("buffer", 10, "size of the ports channel buffer")
)

func main() {
	log.Println("Starting ports processor application")

	flag.Parse()
	log.Printf("Configuration: storage=%s, filePath=%s, batchSize=%d, bufferSize=%d",
		*storage, *filePath, *batchSize, *chanBufferSize)

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGQUIT,
		os.Interrupt,
	)
	defer stop()

	app, err := app.New(app.AppConfig{
		Storage:        *storage,
		FilePath:       *filePath,
		BatchSize:      *batchSize,
		ChanBufferSize: *chanBufferSize,
	})

	if err != nil {
		log.Fatal("failed to build application: ", err)
	}

	if err := app.Run(ctx); err != nil {
		log.Fatal("failed to run application: ", err)
	}
}
