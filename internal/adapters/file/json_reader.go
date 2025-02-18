package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

type Logger interface {
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

type FileJSONReader struct {
	logger Logger
}

type JSONReader interface {
	StreamPorts(ctx context.Context, filePath string, portsChanSize int) (<-chan Port, <-chan error)
}

var _ JSONReader = (*FileJSONReader)(nil) // Compile-time assertion

func NewJSONReader(logger Logger) *FileJSONReader {
	return &FileJSONReader{
		logger: logger,
	}
}

func (f *FileJSONReader) StreamPorts(ctx context.Context, filePath string, portsChanSize int) (<-chan Port, <-chan error) {
	f.logger.Println("starting streaming ports from file")
	portsChan := make(chan Port, 5)
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
func readPort(ctx context.Context, file *os.File, portsChan chan<- Port, errChan chan<- error) {
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

			var port Port
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
