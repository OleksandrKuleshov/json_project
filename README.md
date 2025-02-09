# Ports Processing Service

A Go microservice that efficiently processes and stores port data from a JSON file, designed to handle large datasets with limited memory resources (200MB).

## Features

- Streaming JSON processing for memory efficiency
- Batch processing of port records
- SQLite in-memory database for storage
- Graceful shutdown handling
- Signal handling (SIGTERM, SIGINT, SIGQUIT)
- Configurable batch and buffer sizes

## Requirements

- Go 1.19 or higher
- Docker (optional)

## Running the Service

### Local Execution

To run the service use `go run cmd/main.go [flags]`
Or in Docker `docker build -t ports-processor .` 
`docker run  ports-processor `

Available flags:
- `-storage`: Storage engine (default: "sqlite")
- `-file`: Path to ports JSON file (default: "../ports.json")
- `-batch`: Batch size for processing (default: 100)
- `-buffer`: Channel buffer size (default: 10)

## Testing
There are tests for port_service and json_reader.
To run them, use: `go test ./...` in root folder

## Architecture

The project follows hexagonal architecture principles with:

- Core domain logic in `internal/core`
- Adapters for external interactions in `internal/adapters`
- Clear separation of concerns through interfaces
- Domain-driven design patterns

## Design Decisions

1. **Streaming Processing**: Uses Go channels to process large JSON files without loading them entirely into memory
2. **Batch Processing**: Groups port records into batches for efficient database operations
3. **In-Memory SQLite**: Chosen for simplicity while maintaining ACID compliance
4. **Error Handling**: Comprehensive error handling with graceful shutdown capabilities

## Limitations

- Currently only supports SQLite storage
- In-memory database means data is not persisted between restarts