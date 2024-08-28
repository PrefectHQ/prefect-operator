package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func Hash(obj interface{}, length int) (string, error) {
	data, err := json.Marshal(obj) // Serialize the object to JSON
	if err != nil {
		return "", err
	}

	hash := sha256.New()
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))[:length], nil
}
