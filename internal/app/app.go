package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/OleksandrKuleshov/home-task/internal/adapters/file"
	"github.com/OleksandrKuleshov/home-task/internal/adapters/repository"
	"github.com/OleksandrKuleshov/home-task/internal/core/services"
)

type AppConfig struct {
	Storage        string
	FilePath       string
	BatchSize      int
	ChanBufferSize int
}

type Application struct {
	service    services.PortService
	repository repository.PortRepository
	config     AppConfig
}

func New(config AppConfig) (*Application, error) {
	var repoInstance repository.PortRepository

	var err error
	if config.Storage == "sqlite" {
		repoLogger := log.New(os.Stdout, "REPOSITORY: ", log.Ldate|log.Ltime|log.Lshortfile)
		repoInstance, err = repository.NewSQLiteRepoistory(repoLogger)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Fatalf("storage: %s not implemented", config.Storage)
	}

	jsonReaderLogger := log.New(os.Stdout, "JSON_READER: ", log.Ldate|log.Ltime|log.Lshortfile)
	jsonReader := file.NewJSONReader(jsonReaderLogger)

	serviceLogger := log.New(os.Stdout, "SERVICE: ", log.Ldate|log.Ltime|log.Lshortfile)

	return &Application{
		service:    services.NewPortsService(repoInstance, jsonReader, serviceLogger),
		repository: repoInstance,
		config:     config,
	}, nil
}

func (app *Application) Run(ctx context.Context) error {
	processingDone := make(chan error)

	config := services.Config{
		FilePath:       app.config.FilePath,
		BatchSize:      app.config.BatchSize,
		ChanBufferSize: app.config.ChanBufferSize,
	}

	go func() {
		defer close(processingDone)
		processingDone <- app.service.ReadAndStorePorts(ctx, config)
	}()

	select {
	case err := <-processingDone:
		shutdownErr := app.shutdown()

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
		shutdownErr := app.shutdown()
		if shutdownErr != nil {
			log.Printf("shutdown error during cancellation: %v", shutdownErr)
		}
		log.Print(ctx.Err())
		os.Exit(1)
	}

	return nil
}

func (app *Application) shutdown() error {
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
