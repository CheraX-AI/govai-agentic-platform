package recovery

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestPasswordValidationAndRedaction(t *testing.T) {
	for _, value := range []string{"12345678", " secret ", "தமிழ்மொழி123"} {
		p, err := ParsePassword(value)
		if err != nil || p.Reveal() != value {
			t.Fatal("valid password changed/rejected")
		}

		b, _ := json.Marshal(p)
		if strings.Contains(string(b)+fmt.Sprintf("%v %+v %#v", p, p, p)+p.LogValue().String(), value) {
			t.Fatal("password disclosure")
		}
	}

	for _, value := range []string{
		"",
		"1234567",
		strings.Repeat("x", 1025),
		"12345678\x00",
		string([]byte{255, 255, 255, 255, 255, 255, 255, 255}),
	} {
		if _, err := ParsePassword(value); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
}
