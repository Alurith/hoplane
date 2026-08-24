package domain

import "testing"

func TestNewConnectionIDIsOpaqueAndValid(t *testing.T) {
	first, err := NewConnectionID()
	if err != nil {
		t.Fatalf("NewConnectionID() error = %v", err)
	}
	second, err := NewConnectionID()
	if err != nil {
		t.Fatalf("NewConnectionID() error = %v", err)
	}
	if first == second {
		t.Fatal("generated duplicate connection IDs")
	}
	if err := ValidateConnectionID(first); err != nil {
		t.Fatalf("ValidateConnectionID() error = %v", err)
	}
}

func TestValidateConnectionIDRejectsAmbiguousValues(t *testing.T) {
	for _, value := range []string{
		"",
		"office",
		" 0123456789abcdef0123456789abcdef",
		"0123456789abcdef0123456789abcdef ",
		"0123456789abcdef0123456789abcdeg",
		"0123456789abcdef0123456789abcdef0",
	} {
		if err := ValidateConnectionID(value); err == nil {
			t.Fatalf("ValidateConnectionID(%q) = nil, want error", value)
		}
	}
}
