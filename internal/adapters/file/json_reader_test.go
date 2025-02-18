package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

type mockLogger struct {
	logs []string
}

func (m *mockLogger) Println(v ...interface{}) {
	m.logs = append(m.logs, fmt.Sprintln(v...))
}

func (m *mockLogger) Printf(format string, v ...interface{}) {
	m.logs = append(m.logs, fmt.Sprintf(format, v...))
}

func TestFileJSONReader_StreamPorts(t *testing.T) {

	testJSON := `{
		"TEST1": {
			"name": "Test Port 1",
			"city": "Test City",
			"country": "Test Country"
		},
		"TEST2": {
			"name": "Test Port 2",
			"city": "Test City 2",
			"country": "Test Country 2"
		}
	}`

	tmpfile, err := os.CreateTemp("", "ports*.json")

	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(testJSON)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		filePath      string
		portsChanSize int
		expectedPorts int
		expectedError error
		setupContext  func() (context.Context, context.CancelFunc)
	}{
		{
			name:          "successful read",
			filePath:      tmpfile.Name(),
			portsChanSize: 5,
			expectedPorts: 2,
			expectedError: nil,
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 1*time.Second)
			},
		},

		{
			name:          "file not found",
			filePath:      "nonexistent.json",
			portsChanSize: 5,
			expectedPorts: 0,
			expectedError: os.ErrNotExist,
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 1*time.Second)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			reader := NewJSONReader(logger)
			ctx, cancel := tt.setupContext()
			defer cancel()

			portsChan, errChan := reader.StreamPorts(ctx, tt.filePath, tt.portsChanSize)

			var portsCount int
			var lastErr error

			for {
				select {
				case _, ok := <-portsChan:
					if !ok {
						portsChan = nil
					} else {
						portsCount++
					}
				case err, ok := <-errChan:
					if !ok {
						errChan = nil
					} else {
						lastErr = err
					}
				}
				if portsChan == nil && errChan == nil {
					break
				}
			}

			if portsCount != tt.expectedPorts {
				t.Errorf("expected %d ports, got %d", tt.expectedPorts, portsCount)
			}

			if tt.expectedError != nil {
				if lastErr == nil {
					t.Errorf("expected error %v but got nil", tt.expectedError)
				} else if !errors.Is(lastErr, tt.expectedError) {
					t.Errorf("expected error: %v but got error: %v", tt.expectedError, lastErr)
				}
			} else if lastErr != nil {
				t.Errorf("expected no error but got: %v", lastErr)
			}
		})
	}
}
