package conversation

// Open decrypts a stored message, so tests can check what was saved.
func Open(key, sealed []byte, conversationID, messageID, role string) (string, error) {
	s, err := newSealer(key)
	if err != nil {
		return "", err
	}

	return s.open(sealed, conversationID, messageID, role)
}
