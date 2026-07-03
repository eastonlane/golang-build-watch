package version

import "testing"

func TestStringFallsBackToDefault(t *testing.T) {
	previous := Value
	Value = ""
	t.Cleanup(func() {
		Value = previous
	})

	if got := String(); got != defaultValue {
		t.Fatalf("String() = %q, want %q", got, defaultValue)
	}
}

func TestStringReturnsInjectedValue(t *testing.T) {
	previous := Value
	Value = "abc123"
	t.Cleanup(func() {
		Value = previous
	})

	if got := String(); got != "abc123" {
		t.Fatalf("String() = %q, want abc123", got)
	}
}
