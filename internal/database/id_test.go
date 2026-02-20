package database

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestGenerateID(t *testing.T) {
	id := GenerateID()

	// Must be valid UUID
	parsed, err := uuid.Parse(id)
	if err != nil {
		t.Fatalf("GenerateID() returned invalid UUID: %s, error: %v", id, err)
	}

	// Must be version 7
	if parsed.Version() != 7 {
		t.Errorf("GenerateID() version = %d, want 7", parsed.Version())
	}

	// Must be lowercase with hyphens
	if id != strings.ToLower(id) {
		t.Errorf("GenerateID() not lowercase: %s", id)
	}

	// Two IDs must be different
	id2 := GenerateID()
	if id == id2 {
		t.Errorf("GenerateID() returned duplicate: %s", id)
	}

	// Second ID should be >= first (time-ordered)
	if id2 < id {
		t.Errorf("GenerateID() not time-ordered: %s < %s", id2, id)
	}
}
