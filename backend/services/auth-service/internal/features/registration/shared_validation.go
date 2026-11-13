package registration

import (
	"encoding/json"
	"unicode/utf8"
)

// CanonicalEmail applies the agreed ASCII new-account email policy. It does not
// rewrite a stored identity, perform alias folding or prove address ownership.
func CanonicalEmail(value string) string {
	if len(value) > 254 {
		return ""
	}

	return normalizeEmail(value)
}

// ValidJSONEncoding prevents replacement of invalid UTF-8/lone surrogates in
// sensitive credentials before the JSON decoder touches them.
func ValidJSONEncoding(body []byte) bool {
	return utf8.Valid(body) && json.Valid(body) && validSurrogates(body)
}
