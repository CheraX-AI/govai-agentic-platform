package recovery

import "testing"

func TestMaskEmail(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"citizen@example.test", "ci*****@example.test"},
		{"a@example.test", "*****@example.test"},
		{"ab@example.test", "*****@example.test"},
		{"தமிழ்@example.test", "தம*****@example.test"},
	} {
		got, err := MaskEmail(c.in)
		if err != nil || got != c.want {
			t.Fatalf("mask mismatch: %v", err)
		}
	}
	for _, value := range []string{"", "invalid", "Name <citizen@example.test>", "citizen@example.test\r\nBcc: other@example.test", string([]byte{255}) + "@example.test"} {
		if got, err := MaskEmail(value); err == nil || got != "" {
			t.Fatal("unsafe mask input accepted")
		}
	}
}
