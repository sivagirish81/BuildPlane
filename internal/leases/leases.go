package leases

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"time"

	"github.com/buildplane/buildplane/internal/database"
	"github.com/buildplane/buildplane/internal/domain"
	"github.com/google/uuid"
)

func NewToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(raw)
	return token, Hash(token), nil
}

func Hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

var ErrRejected = errors.New("lease update rejected")

func ValidateAttempt(ctx context.Context, store *database.Store, jobID, attemptID uuid.UUID, token string) (domain.Attempt, error) {
	attempt, err := store.AttemptByID(ctx, attemptID)
	if err != nil {
		return domain.Attempt{}, err
	}
	if attempt.JobID != jobID {
		return domain.Attempt{}, ErrRejected
	}
	latest, err := store.LatestAttemptNumber(ctx, jobID)
	if err != nil {
		return domain.Attempt{}, err
	}
	if latest != attempt.AttemptNumber {
		return domain.Attempt{}, ErrRejected
	}
	if attempt.Status != domain.StatusScheduled && attempt.Status != domain.StatusRunning {
		return domain.Attempt{}, ErrRejected
	}
	if time.Now().After(attempt.LeaseExpiresAt) {
		return domain.Attempt{}, ErrRejected
	}
	if subtle.ConstantTimeCompare([]byte(Hash(token)), []byte(attempt.LeaseTokenHash)) != 1 {
		return domain.Attempt{}, ErrRejected
	}
	return attempt, nil
}
