package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/OleksandrKuleshov/home-task/internal/adapters/file"
	"github.com/OleksandrKuleshov/home-task/internal/adapters/repository"
	"github.com/OleksandrKuleshov/home-task/internal/core/domain"
)

type contextKey string

const (
	FilePath       contextKey = "filePath"
	BatchSize      contextKey = "batchSize"
	ChanBufferSize contextKey = "buffer"
)

type Config struct {
	FilePath       string
	BatchSize      int
	ChanBufferSize int
}

type Logger interface {
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

type PortsService struct {
	repo       repository.PortRepository
	jsonReader file.JSONReader
	logger     Logger
}

type PortService interface {
	ReadAndStorePorts(ctx context.Context, conf Config) error
}

var _ PortService = (*PortsService)(nil) // Compile-time assertion

func NewPortsService(repo repository.PortRepository, jsonReader file.JSONReader, logger Logger) *PortsService {
	return &PortsService{
		repo:       repo,
		jsonReader: jsonReader,
		logger:     logger,
	}
}

// collectPortBatches accumulates ports into batches of specified size
// before sending them for processing. This helps optimize memory use and database operations
func (s *PortsService) ReadAndStorePorts(ctx context.Context, config Config) error {
	s.logger.Println("starting readAndStore ports process")

	filePath := config.FilePath
	if filePath == "" {
		return fmt.Errorf("filePath not provided")
	}

	batchSize := config.BatchSize
	if batchSize <= 0 {
		return fmt.Errorf("invalid batch size")
	}

	chanBufferSize := config.ChanBufferSize
	if chanBufferSize <= 0 {
		return fmt.Errorf("invalid buffer size")
	}

	processingErrChan := make(chan error, 2)
	portsChan, readerErrChan := s.jsonReader.StreamPorts(ctx, filePath, chanBufferSize)
	batchChan := make(chan []domain.Port)

	var wg sync.WaitGroup
	wg.Add(2)

	s.logger.Printf("starting batch processing with size: %d", batchSize)

	go collectPortBatches(ctx, portsChan, batchChan, batchSize, &wg)
	go s.savePortBatches(ctx, batchChan, processingErrChan, &wg)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
		close(processingErrChan)
	}()

	cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelCleanup()
	select {
	case <-ctx.Done():
		s.logger.Println("shutdown signal received, waiting for in-progress operations...")

		select {
		case <-done:
			s.logger.Println("all operations completed successfully")
			return ctx.Err()
		case <-cleanupCtx.Done():
			return fmt.Errorf("shutdown timed out waiting for operations: %w", ctx.Err())
		}

	case err := <-readerErrChan:
		if err != nil {
			if err == context.Canceled || err == context.DeadlineExceeded {
				return err
			}
			return fmt.Errorf("error reading ports: %w", err)
		}

		select {
		case err := <-processingErrChan:
			if err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					return err
				}
				return fmt.Errorf("error processing ports: %w", err)
			}
		case <-done:
			return nil
		case <-ctx.Done():
			return fmt.Errorf("cancelled during final processing: %w", ctx.Err())
		}

	case err := <-processingErrChan:
		return fmt.Errorf("error processing ports: %w", err)

	}

	return nil
}

func collectPortBatches(
	ctx context.Context,
	portsChan <-chan file.Port,
	batchChan chan<- []domain.Port,
	batchSize int,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	defer close(batchChan)

	batch := make([]domain.Port, 0, batchSize)

	sendBatch := func(ports []domain.Port) bool {
		select {
		case <-ctx.Done():
			return false
		case batchChan <- ports:
			return true
		}
	}

	for {
		select {
		case <-ctx.Done():
			if len(batch) > 0 {
				newBatch := make([]domain.Port, len(batch))
				copy(newBatch, batch)
				sendBatch(newBatch)
			}
			return

		case port, ok := <-portsChan:
			if !ok {
				if len(batch) > 0 {
					newBatch := make([]domain.Port, len(batch))
					copy(newBatch, batch)
					sendBatch(newBatch)
				}
				return
			}

			batch = append(batch, convertFilePortToDomainPort(port))
			if len(batch) >= batchSize {
				newBatch := make([]domain.Port, len(batch))
				copy(newBatch, batch)
				if !sendBatch(newBatch) {
					return
				}
				batch = batch[:0]
			}
		}
	}
}

func (s *PortsService) savePortBatches(ctx context.Context, batchChan <-chan []domain.Port, errChan chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case batch, ok := <-batchChan:
			if !ok {
				return
			}
			portsDB := make([]repository.PortDB, len(batch))
			for i, port := range batch {
				portsDB[i] = convertPortToPortDB(port)
			}
			s.logger.Printf("processing batch of %d ports", len(batch))
			err := s.repo.UpsertPorts(ctx, portsDB)
			if err != nil {
				errChan <- fmt.Errorf("failed to upsert ports: %w", err)
			}
		}
	}
}

func convertPortToPortDB(port domain.Port) repository.PortDB {
	return repository.PortDB{
		Key:         port.Key,
		Name:        port.Name,
		City:        port.City,
		Country:     port.Country,
		Alias:       port.Alias,
		Regions:     port.Regions,
		Coordinates: port.Coordinates,
		Province:    port.Province,
		Timezone:    port.Timezone,
		Unlocs:      port.Unlocs,
		Code:        port.Code,
	}
}

func convertFilePortToDomainPort(port file.Port) domain.Port {
	return domain.Port{
		Key:         port.Key,
		Name:        port.Name,
		City:        port.City,
		Country:     port.Country,
		Alias:       port.Alias,
		Regions:     port.Regions,
		Coordinates: port.Coordinates,
		Province:    port.Province,
		Timezone:    port.Timezone,
		Unlocs:      port.Unlocs,
		Code:        port.Code,
	}
}
