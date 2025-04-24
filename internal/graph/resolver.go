package graph

import "genealogy_go/internal/repository"

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	db *repository.DB
}

func NewResolver(db *repository.DB) *Resolver {
	return &Resolver{
		db: db,
	}
} 