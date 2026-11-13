package wiring

import "testing"

func TestOrDefault(t *testing.T) {
	if orDefault(0, 5) != 5 {
		t.Fatal("unset limit must keep the default")
	}

	if orDefault(30, 5) != 30 {
		t.Fatal("configured limit ignored")
	}
}
