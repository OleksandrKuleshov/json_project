package ports

import (
	"context"
	"home-task/internal/core/domain"
)

type Config struct {
	FilePath       string
	BatchSize      int
	ChanBufferSize int
}

type PortService interface {
	ReadAndStorePorts(ctx context.Context, conf Config) error
}

type JSONReader interface {
	StreamPorts(ctx context.Context, filePath string, portsChanSize int) (<-chan domain.Port, <-chan error)
}

type PortRepository interface {
	UpsertPorts(ctx context.Context, ports []domain.Port) error
	Close(ctx context.Context) error
}
