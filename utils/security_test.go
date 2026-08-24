package utils

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestDummyPasswordHashMatchesProductionCost(t *testing.T) {
	cost, err := bcrypt.Cost([]byte(DummyPasswordHash))
	if err != nil {
		t.Fatalf("dummy password hash is invalid: %v", err)
	}
	if cost != bcrypt.DefaultCost {
		t.Fatalf("dummy hash cost = %d, expected %d", cost, bcrypt.DefaultCost)
	}
}
