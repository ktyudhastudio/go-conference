package core

import (
	"encoding/base64"
	"encoding/json"
)

// Encode serializes obj to JSON then base64 (format expected by the UI).
func Encode(obj interface{}) (string, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// Decode reverses Encode.
func Decode(in string, obj interface{}) error {
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, obj)
}
