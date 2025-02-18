package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/OleksandrKuleshov/home-task/internal/adapters/file"
	"github.com/OleksandrKuleshov/home-task/internal/adapters/repository"
)

type mockJSONReader struct {
	streamPortsFn func(ctx context.Context, filePath string, portsChanSize int) (<-chan file.Port, <-chan error)
}

func (m *mockJSONReader) StreamPorts(ctx context.Context, filePath string, portsChanSize int) (<-chan file.Port, <-chan error) {
	return m.streamPortsFn(ctx, filePath, portsChanSize)
}

type mockPortRepository struct {
	upsertFn func(ctx context.Context, ports []repository.PortDB) error
	closeFn  func(ctx context.Context) error
}

func (m *mockPortRepository) UpsertPorts(ctx context.Context, ports []repository.PortDB) error {
	return m.upsertFn(ctx, ports)
}

func (m *mockPortRepository) Close(ctx context.Context) error {
	return nil
}

type mockLogger struct {
	logs []string
}

func (m *mockLogger) Println(v ...interface{}) {
	m.logs = append(m.logs, fmt.Sprintln(v...))
}

func (m *mockLogger) Printf(format string, v ...interface{}) {
	m.logs = append(m.logs, fmt.Sprintf(format, v...))
}

func TestPortService_ReadAndStorePorts(t *testing.T) {

	testPorts := []file.Port{
		{Key: "PORT1", Name: "Test Port 1"},
		{Key: "PORT2", Name: "Test Port 2"},
	}

	tests := []struct {
		name           string
		filePath       string
		batchSize      int
		chanBufferSize int
		setupMocks     func() (file.JSONReader, repository.PortRepository)
		setupContext   func() context.Context
		expectedError  error
	}{
		{
			name:           "filePath not provided",
			filePath:       "",
			batchSize:      2,
			chanBufferSize: 5,
			setupMocks: func() (file.JSONReader, repository.PortRepository) {
				return newMockJSONReader(testPorts, nil), newMockRepository()
			},
			setupContext: func() context.Context {
				return context.Background()
			},
			expectedError: fmt.Errorf("filePath not provided"),
		},

		{
			name:           "invalid batch size",
			filePath:       "test-file-path.json",
			batchSize:      0,
			chanBufferSize: 5,
			setupMocks: func() (file.JSONReader, repository.PortRepository) {
				return newMockJSONReader(testPorts, nil), newMockRepository()
			},
			setupContext: func() context.Context {
				return context.Background()
			},
			expectedError: fmt.Errorf("invalid batch size"),
		},

		{
			name:           "invalid buffer size",
			filePath:       "test-file-path.json",
			batchSize:      5,
			chanBufferSize: 0,
			setupMocks: func() (file.JSONReader, repository.PortRepository) {
				return newMockJSONReader(testPorts, nil), newMockRepository()
			},
			setupContext: func() context.Context {
				return context.Background()
			},
			expectedError: fmt.Errorf("invalid buffer size"),
		},
		{
			name:           "successful processing of ports",
			filePath:       "test.json",
			batchSize:      2,
			chanBufferSize: 5,
			setupMocks: func() (file.JSONReader, repository.PortRepository) {
				return newMockJSONReader(testPorts, nil), newMockRepository()
			},
			setupContext: func() context.Context {
				return context.Background()
			},
			expectedError: nil,
		},
		{
			name:           "reader returns error",
			filePath:       "test.json",
			batchSize:      2,
			chanBufferSize: 5,
			setupMocks: func() (file.JSONReader, repository.PortRepository) {
				return newMockJSONReader(nil, errors.New("read error")), newMockRepository()
			},
			expectedError: errors.New("error reading ports: read error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonReader, repo := tt.setupMocks()
			logger := &mockLogger{}
			service := NewPortsService(repo, jsonReader, logger)

			ctx := context.Background()
			config := Config{
				FilePath:       tt.filePath,
				BatchSize:      tt.batchSize,
				ChanBufferSize: tt.chanBufferSize,
			}
			err := service.ReadAndStorePorts(ctx, config)

			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("expected error %v but got nil", tt.expectedError)
				} else if err.Error() != tt.expectedError.Error() {
					t.Errorf("expected error: %v but got error: %v", tt.expectedError, err)
				}
			} else if err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}

func TestPortService_MemoryConstraints(t *testing.T) {

	size := 100000

	tmpfile, err := os.CreateTemp("", "ports.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.WriteString("{\n")
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < size; i++ {
		portEntry := fmt.Sprintf(`"PORT%d": {
            "name": "Test Port %d",
            "city": "Test City",
            "country": "Test Country",
            "alias": ["alias1", "alias2"],
            "regions": ["region1", "region2"],
            "coordinates": [1.0, 1.0],
            "province": "Test Province",
            "timezone": "UTC",
            "unlocs": ["unloc1", "unloc2"],
            "code": "CODE%d"
        }`, i, i, i)

		if i < size-1 {
			portEntry += ","
		}

		if _, err := tmpfile.WriteString(portEntry); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := tmpfile.WriteString("\n}"); err != nil {
		t.Fatal(err)
	}

	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	t.Run("memory usage stays within limits", func(t *testing.T) {
		var m1, m2 runtime.MemStats

		runtime.GC()
		runtime.ReadMemStats(&m1)
		logger := &mockLogger{}

		repo, err := repository.NewSQLiteRepoistory(logger)
		if err != nil {
			t.Fatal(err)
		}
		defer repo.Close(context.Background())

		reader := file.NewJSONReader(logger)
		service := NewPortsService(repo, reader, logger)

		ctx := context.Background()
		err = service.ReadAndStorePorts(ctx, Config{
			FilePath:       tmpfile.Name(),
			BatchSize:      100,
			ChanBufferSize: 10,
		})

		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}

		runtime.ReadMemStats(&m2)

		t.Logf("Initial HeapAlloc: %d bytes", m1.HeapAlloc)
		t.Logf("Final HeapAlloc: %d bytes", m2.HeapAlloc)
		t.Logf("Initial Alloc: %d bytes", m1.Alloc)
		t.Logf("Final Alloc: %d bytes", m2.Alloc)
		t.Logf("Max Sys: %d bytes", m2.Sys)

		var memoryDiff uint64
		if m2.HeapAlloc > m1.HeapAlloc {
			memoryDiff = m2.HeapAlloc - m1.HeapAlloc
		} else {
			memoryDiff = 0
		}

		const maxMemoryUsage = uint64(200 * 1024 * 1024)
		if memoryDiff > maxMemoryUsage {
			t.Errorf("Memory usage exceeded 200MB limit: %d bytes used", memoryDiff)
		}
		t.Logf("Memory difference: %d bytes", memoryDiff)
	})
}

func TestPortService_SignalHandling(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "ports*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString("{\n"); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 100000; i++ {
		portEntry := fmt.Sprintf(`"PORT%d": {
            "name": "Test Port %d",
            "city": "Test City",
            "country": "Test Country",
            "coordinates": [1.0, 1.0]
        }`, i, i)

		if i < 99999 {
			portEntry += ","
		}

		if _, err := tmpfile.WriteString(portEntry); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := tmpfile.WriteString("\n}"); err != nil {
		t.Fatal(err)
	}

	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name              string
		singal            os.Signal
		expectedErrorText string
	}{
		{
			name:              "handles SIGTERM gracefully",
			singal:            syscall.SIGTERM,
			expectedErrorText: "context canceled",
		},
		{
			name:              "handles SIGINT gracefully",
			singal:            syscall.SIGINT,
			expectedErrorText: "context canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			ctx, cancel := context.WithCancel(context.Background())

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, tt.singal)

			repo, err := repository.NewSQLiteRepoistory(logger)
			if err != nil {
				t.Fatal(err)
			}
			defer repo.Close(context.Background())

			reader := file.NewJSONReader(logger)
			service := NewPortsService(repo, reader, logger)

			errChan := make(chan error, 1)

			go func() {
				err := service.ReadAndStorePorts(ctx, Config{
					FilePath:       tmpfile.Name(),
					BatchSize:      100,
					ChanBufferSize: 10,
				})
				errChan <- err
			}()

			go func() {
				select {
				case <-sigChan:
					cancel()
				case <-ctx.Done():
					return
				}
			}()

			time.Sleep(100 * time.Millisecond)

			proc, err := os.FindProcess(os.Getpid())
			if err != nil {
				t.Fatal(err)
			}
			if err := proc.Signal(tt.singal); err != nil {
				t.Fatal(err)
			}

			select {
			case err := <-errChan:
				if err == nil {
					t.Errorf("expected error but got nil")
				} else if !strings.Contains(err.Error(), tt.expectedErrorText) {
					t.Errorf("expected error text to contain: %q, got: %q", tt.expectedErrorText, err.Error())
				}
			case <-time.After(5 * time.Second):
				t.Fatal("timeout waiting for graceful shutdown")
			}

			signal.Stop(sigChan)
			close(sigChan)
		})
	}
}

func newMockJSONReader(ports []file.Port, err error) *mockJSONReader {
	return &mockJSONReader{
		streamPortsFn: func(ctx context.Context, filePath string, portsChanSize int) (<-chan file.Port, <-chan error) {
			portsChan := make(chan file.Port, len(ports))
			errChan := make(chan error, 1)

			go func() {
				defer close(portsChan)
				defer close(errChan)

				if err != nil {
					errChan <- err
					return
				}

				for _, port := range ports {
					select {
					case <-ctx.Done():
						errChan <- ctx.Err()
						return
					case portsChan <- port:
					}
				}
			}()

			return portsChan, errChan
		},
	}
}

func newMockRepository() *mockPortRepository {

	repo := &mockPortRepository{
		upsertFn: func(ctx context.Context, ports []repository.PortDB) error {
			return nil
		},
		closeFn: func(ctx context.Context) error {
			return nil
		},
	}

	return repo
}
