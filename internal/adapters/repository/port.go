package repository

import "github.com/google/uuid"

type PortDB struct {
	ID          uuid.UUID `db:"id"`
	Key         string    `db:"key"`
	Name        string    `db:"name"`
	City        string    `db:"city"`
	Country     string    `db:"country"`
	Alias       []string  `db:"alias"`       // JSON string
	Regions     []string  `db:"regions"`     // JSON string
	Coordinates []float64 `db:"coordinates"` // JSON string
	Province    string    `db:"province"`
	Timezone    string    `db:"timezone"`
	Unlocs      []string  `db:"unlocs"` // JSON string
	Code        string    `db:"code"`
}
