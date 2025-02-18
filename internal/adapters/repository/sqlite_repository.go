package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	_ "github.com/mattn/go-sqlite3"
)

type Logger interface {
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

type PortSQLiteRepository struct {
	db     *sql.DB
	logger Logger
}

type PortRepository interface {
	UpsertPorts(ctx context.Context, ports []PortDB) error
	Close(ctx context.Context) error
}

var _ PortRepository = (*PortSQLiteRepository)(nil)

var ErrFailedOpenDBConnection = errors.New("failed to open connection to database: ")

func NewSQLiteRepoistory(logger Logger) (*PortSQLiteRepository, error) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("%s, %w", ErrFailedOpenDBConnection, err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	repo := &PortSQLiteRepository{
		db:     db,
		logger: logger,
	}

	if err := repo.initialize(); err != nil {
		db.Close()
		return nil, err
	}

	return repo, nil
}

func (r *PortSQLiteRepository) initialize() error {
	if err := createPortsTable(r.db); err != nil {
		return fmt.Errorf("failed to create ports table: %w", err)
	}
	return nil
}

func (r *PortSQLiteRepository) UpsertPorts(ctx context.Context, ports []PortDB) error {
	r.logger.Printf("starting batch upsert of %d ports", len(ports))

	if err := ctx.Err(); err != nil {
		return err
	}

	fail := func(err error) error {
		r.logger.Printf("upsert failed: %v", err)
		return fmt.Errorf("UpsertPorts: %v", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fail(err)
	}
	defer func() {
		if err := ctx.Err(); err != nil {
			tx.Rollback()
		}
	}()

	valueStrings := make([]string, 0, len(ports))
	valueArgs := make([]interface{}, 0, len(ports)*12)
	//since number of ports in batch is dynamic, we iterate over x ports and add them one by one to the query
	for _, port := range ports {
		if err := ctx.Err(); err != nil {
			return err
		}
		valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")

		aliasJSON, err := json.Marshal(port.Alias)
		if err != nil {
			return fail(err)
		}
		regionsJSON, err := json.Marshal(port.Regions)
		if err != nil {
			return fail(err)
		}
		coordsJSON, err := json.Marshal(port.Coordinates)
		if err != nil {
			return fail(err)
		}
		unlocsJSON, err := json.Marshal(port.Unlocs)
		if err != nil {
			return fail(err)
		}

		valueArgs = append(valueArgs,
			uuid.New(),
			port.Key,
			port.Name,
			port.City,
			port.Country,
			string(aliasJSON),
			string(regionsJSON),
			string(coordsJSON),
			port.Province,
			port.Timezone,
			string(unlocsJSON),
			port.Code,
		)
	}

	query := fmt.Sprintf(`
	    INSERT INTO ports (
	        id, key, name, city, country, alias, regions, coordinates,
	        province, timezone, unlocs, code
	    ) VALUES %s
	    ON CONFLICT(key) DO UPDATE SET
	        key = excluded.key,
	        name = excluded.name,
	        city = excluded.city,
	        country = excluded.country,
	        alias = excluded.alias,
	        regions = excluded.regions,
	        coordinates = excluded.coordinates,
	        province = excluded.province,
	        timezone = excluded.timezone,
	        unlocs = excluded.unlocs,
	        code = excluded.code
	`, strings.Join(valueStrings, ","))

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fail(err)
	}

	_, err = stmt.ExecContext(ctx, valueArgs...)
	if err != nil {
		return fail(err)
	}

	if err = tx.Commit(); err != nil {
		return fail(err)
	}

	r.logger.Printf("successfully upserted batch of %d ports", len(ports))
	return nil
}

func (r *PortSQLiteRepository) Close(ctx context.Context) error {
	r.logger.Println("closing database connection")

	done := make(chan error, 1)
	go func() {
		if err := r.db.Close(); err != nil {
			done <- fmt.Errorf("error closing database: %w", err)
			return
		}
		done <- nil
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("could not close db: %w", err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while closing db: %w", ctx.Err())
	}
}

func createPortsTable(db *sql.DB) error {
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS ports (
		id UUID PRIMARY KEY,
		key TEXT UNIQUE,
		name TEXT,
		city TEXT,
		country TEXT,
		alias TEXT,
		regions TEXT,
		coordinates TEXT,
		province TEXT,
		timezone TEXT,
		unlocs TEXT,
		code TEXT 
	);`

	_, err := db.Exec(createTableQuery)
	if err != nil {
		return fmt.Errorf("failed to execute create table SQL: %w", err)
	}
	return nil
}
