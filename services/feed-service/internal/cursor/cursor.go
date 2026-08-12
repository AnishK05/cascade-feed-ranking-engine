// Package cursor encodes and validates opaque keyset pagination cursors.
package cursor

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
)

const version = 1

type Cursor struct {
	Score           float64 `json:"s"`
	CreatedAtUnixMs int64   `json:"t"`
	PostID          int64   `json:"i"`
	Version         int     `json:"v"`
}

func Encode(score float64, createdAtUnixMs, postID int64) (string, error) {
	if !valid(score, createdAtUnixMs, postID) {
		return "", fmt.Errorf("invalid cursor values")
	}
	encoded, err := json.Marshal(Cursor{
		Score: score, CreatedAtUnixMs: createdAtUnixMs, PostID: postID, Version: version,
	})
	if err != nil {
		return "", fmt.Errorf("marshal cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func Decode(token string) (Cursor, error) {
	if token == "" || len(token) > 1024 {
		return Cursor{}, fmt.Errorf("invalid page token")
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return Cursor{}, fmt.Errorf("decode page token: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value Cursor
	if err := decoder.Decode(&value); err != nil {
		return Cursor{}, fmt.Errorf("decode cursor: %w", err)
	}
	if err := ensureEOF(decoder); err != nil {
		return Cursor{}, err
	}
	if value.Version != version || !valid(value.Score, value.CreatedAtUnixMs, value.PostID) {
		return Cursor{}, fmt.Errorf("invalid cursor values")
	}
	return value, nil
}

func valid(score float64, createdAtUnixMs, postID int64) bool {
	return !math.IsNaN(score) && !math.IsInf(score, 0) && createdAtUnixMs > 0 && postID > 0
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("page token contains trailing data")
		}
		return fmt.Errorf("decode trailing page token data: %w", err)
	}
	return nil
}
