package authn

import "errors"

// ErrConfig means the verifier settings are unsafe or incomplete.
var ErrConfig = errors.New("invalid access-token verifier configuration")

// ErrToken means the access token was missing, malformed or failed a check.
var ErrToken = errors.New("invalid access token")
