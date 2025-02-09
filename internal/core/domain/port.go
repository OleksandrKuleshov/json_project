package domain

import "github.com/google/uuid"

type Port struct {
	Key         string    `json:"-"`
	Name        string    `json:"name"`
	City        string    `json:"city"`
	Country     string    `json:"country"`
	Alias       []string  `json:"alias"`
	Regions     []string  `json:"regions"`
	Coordinates []float64 `json:"coordinates"`
	Province    string    `json:"province"`
	Timezone    string    `json:"timezone"`
	Unlocs      []string  `json:"unlocs"`
	Code        string    `json:"code"`
}

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
