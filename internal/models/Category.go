package models

type Category struct {
	ID       string `json:"id"`
	UserID   string `json:"user_id"`
	Name     string `json:"name"`
	IsSystem bool   `json:"is_system"`
}
