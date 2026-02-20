package database

import "github.com/google/uuid"

// GenerateID returns a new UUIDv7 string.
// UUIDv7 is time-ordered, making it suitable for primary keys
// with better index locality than random UUIDs.
func GenerateID() string {
	return uuid.Must(uuid.NewV7()).String()
}
