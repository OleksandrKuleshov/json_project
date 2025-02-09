package main

import (
	"context"
	"flag"
	"fmt"
	"home-task/internal/adapters/file"
	"home-task/internal/adapters/repository"
	"home-task/internal/core/ports"
	"home-task/internal/core/services"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	storage        = flag.String("storage", "sqlite", "storage engine to use")
	filePath       = flag.String("file", "../ports.json", "path to ports JSON file")
	batchSize      = flag.Int("batch", 100, "batch size for ports to process")
	chanBufferSize = flag.Int("buffer", 10, "size of the ports channel buffer")
)

type Application struct {
	service    ports.PortService
	repository ports.PortRepository
}

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

	config := ports.Config{
		FilePath:       *filePath,
		BatchSize:      *batchSize,
		ChanBufferSize: *chanBufferSize,
	}

	app, err := buildApplication()
	if err != nil {
		log.Fatal("failed to build application: ", err)
	}

	processingDone := make(chan error)
	go func() {
		defer close(processingDone)
		processingDone <- app.service.ReadAndStorePorts(ctx, config)
	}()

	select {
	case err := <-processingDone:
		shutdownErr := performApplicationShutdown(app)

		if err != nil {
			log.Printf("service error: %v", err)
			if shutdownErr != nil {
				log.Printf("additional shutdown error: %v", shutdownErr)
			}
			os.Exit(1)
		}

		if shutdownErr != nil {
			log.Printf("shutdown error: %v", shutdownErr)
			os.Exit(1)
		}

		log.Println("Application shutdown complete")
		os.Exit(0)
	case <-ctx.Done():
		shutdownErr := performApplicationShutdown(app)
		if shutdownErr != nil {
			log.Printf("shutdown error during cancellation: %v", shutdownErr)
		}
		log.Print(ctx.Err())
		os.Exit(1)
	}
}

func buildApplication() (*Application, error) {

	var repoInstance ports.PortRepository

	var err error
	if *storage == "sqlite" {
		repoInstance, err = repository.NewSQLiteRepoistory()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Fatalf("storage: %s not implemented", *storage)
	}

	jsonReader := file.NewJSONReader()

	return &Application{
		service:    services.NewPortsService(repoInstance, jsonReader),
		repository: repoInstance,
	}, nil
}

func performApplicationShutdown(app *Application) error {
	log.Println("initiating graceful shutdown")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	shutdownErrs := make(chan error, 2)

	go func() {
		if err := app.repository.Close(shutdownCtx); err != nil {
			shutdownErrs <- fmt.Errorf("failed to close repository: %w", err)
			return
		}
		shutdownErrs <- nil
	}()

	select {
	case err := <-shutdownErrs:
		if err != nil {
			return fmt.Errorf("shutdown error: %w", err)
		}
		log.Println("shutdown completed successfully")
		return nil
	case <-shutdownCtx.Done():
		return fmt.Errorf("shutdown timed out: %w", shutdownCtx.Err())
	}
}
