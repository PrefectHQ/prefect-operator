package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// Hash returns a hashed string based on an input object,
// which is JSON serialized, and the length of the output.
func Hash(obj interface{}, length int) (string, error) {
	data, err := json.Marshal(obj) // Serialize the object to JSON
	if err != nil {
		return "", err
	}

	hash := sha256.New()
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))[:length], nil
}
