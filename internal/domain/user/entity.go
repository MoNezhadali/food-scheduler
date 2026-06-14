package user

import "time"

type Preferences struct {
	ExcludedAllergens   []string
	DietaryRestrictions []string // e.g. "vegetarian", "no-pork"
}

type User struct {
	ID           string
	Email        string
	PasswordHash string
	Preferences  Preferences
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
