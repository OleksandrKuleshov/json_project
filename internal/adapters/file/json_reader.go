package file

import (
	"context"
	"encoding/json"
	"fmt"
	"home-task/internal/core/domain"
	"home-task/internal/core/ports"
	"log"
	"os"
)

type FileJSONReader struct {
	logger *log.Logger
}

var _ ports.JSONReader = (*FileJSONReader)(nil) // Compile-time assertion

func NewJSONReader() *FileJSONReader {
	return &FileJSONReader{
		logger: log.New(os.Stdout, "JSON_READER: ", log.Ldate|log.Ltime|log.Lshortfile),
	}
}

func (f *FileJSONReader) StreamPorts(ctx context.Context, filePath string, portsChanSize int) (<-chan domain.Port, <-chan error) {
	f.logger.Println("starting streaming ports from file")
	portsChan := make(chan domain.Port, 5)
	errChan := make(chan error, 1)

	file, err := os.Open(filePath)
	if err != nil {
		f.logger.Printf("failed to open file %s: %v", filePath, err)
		errChan <- err
		close(portsChan)
		close(errChan)
		return portsChan, errChan
	}

	go func() {
		defer close(portsChan)
		defer close(errChan)
		defer file.Close()

		defer f.logger.Println("done streaming ports from file")
		readPort(ctx, file, portsChan, errChan)
	}()

	return portsChan, errChan
}

// readPort processes the JSON file as a stream of tokens
// This allows processing large files without loading them entirely into memory
func readPort(ctx context.Context, file *os.File, portsChan chan<- domain.Port, errChan chan<- error) {
	decoder := json.NewDecoder(file)

	_, err := decoder.Token()
	if err != nil {
		errChan <- fmt.Errorf("failed to read initial token: %w", err)
		return
	}

	for decoder.More() {
		select {
		case <-ctx.Done():
			errChan <- ctx.Err()
			return
		default:
			key, err := decoder.Token()
			if err != nil {
				errChan <- fmt.Errorf("failed to read port key: %w", err)
				return
			}

			var port domain.Port
			err = decoder.Decode(&port)
			if err != nil {
				errChan <- fmt.Errorf("failed to decode port: %w", err)
				return
			}
			port.Key = key.(string)

			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			case portsChan <- port:
			}
		}
	}
}
