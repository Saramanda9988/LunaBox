package models

import "time"

type Category struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	IsSystem  bool      `json:"is_system"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
