package contentsync

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	HeaderTimestamp  = "X-Unigo-Timestamp"
	HeaderRunID      = "X-Unigo-Run-Id"
	HeaderRepository = "X-Unigo-Repository"
	HeaderSignature  = "X-Unigo-Signature"
	MaxSkew          = 5 * time.Minute
)

func bodyHash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func canonicalString(method, path, timestamp, runID, repository, bodySHA string) string {
	return strings.Join([]string{
		strings.ToUpper(method),
		path,
		timestamp,
		runID,
		repository,
		bodySHA,
	}, "\n")
}

// Sign returns the hex HMAC-SHA-256 of the canonical request string.
func Sign(secret, method, path, timestamp, runID, repository string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(canonicalString(method, path, timestamp, runID, repository, bodyHash(body))))
	return hex.EncodeToString(mac.Sum(nil))
}

func parseSignature(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "sha256=")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("%w: missing signature", ErrUnauthorized)
	}
	got, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid signature", ErrUnauthorized)
	}
	return got, nil
}

// VerifyHMAC checks timestamp skew and the HMAC over method/path/timestamp/run/repo/body.
func VerifyHMAC(secret, method, path, timestamp, runID, repository, signature string, body []byte, now time.Time) error {
	if strings.TrimSpace(secret) == "" {
		return ErrNotConfigured
	}
	if strings.TrimSpace(runID) == "" || strings.TrimSpace(repository) == "" {
		return fmt.Errorf("%w: run id and repository are required", ErrUnauthorized)
	}
	unix, err := strconv.ParseInt(strings.TrimSpace(timestamp), 10, 64)
	if err != nil {
		return fmt.Errorf("%w: invalid timestamp", ErrStaleTimestamp)
	}
	ts := time.Unix(unix, 0)
	if now.IsZero() {
		now = time.Now()
	}
	delta := now.Sub(ts)
	if delta < 0 {
		delta = -delta
	}
	if delta > MaxSkew {
		return ErrStaleTimestamp
	}
	want, err := hex.DecodeString(Sign(secret, method, path, strings.TrimSpace(timestamp), runID, repository, body))
	if err != nil {
		return fmt.Errorf("%w: sign", ErrUnauthorized)
	}
	got, err := parseSignature(signature)
	if err != nil {
		return err
	}
	if !hmac.Equal(want, got) {
		return ErrUnauthorized
	}
	return nil
}
