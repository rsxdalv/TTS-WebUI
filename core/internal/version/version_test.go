package version

import "testing"

func TestStringContainsVersion(t *testing.T) {
	if got := String(); got == "" {
		t.Fatal("String() returned an empty string")
	}
}
