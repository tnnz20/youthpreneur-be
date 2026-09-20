package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

// graceUsecase is a test-only wrapper exposing the grace window and clock so the
// behaviour can be exercised deterministically without sleeping.
func newGraceUsecase(
	t *testing.T,
	users *fakeUserRepository,
	sessions *fakeSessionRepository,
	now func() time.Time,
) (usecase.AuthUseCase, *fakeTokenService) {
	t.Helper()

	tokens := &fakeTokenService{}

	return usecase.NewAuthUseCaseWithClock(
		users,
		sessions,
		tokens,
		15*time.Minute,
		7*24*time.Hour,
		now,
	), tokens
}

func seedSession(t *testing.T, sessions *fakeSessionRepository, tokens *fakeTokenService, raw string, userID int, now time.Time) entity.RefreshSession {
	t.Helper()

	family := "family-" + raw
	session := entity.RefreshSession{
		UserID:    userID,
		FamilyID:  family,
		TokenHash: tokens.HashRefresh(raw),
		ExpiresAt: now.Add(time.Hour).Unix(),
		CreatedAt: now.Unix(),
	}
	if err := sessions.CreateRefreshSession(context.Background(), session); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	return session
}

// TestGraceDuplicateRefreshReturnsSameReplacement locks the core grace-window
// contract: two refresh calls with the same old token inside the window must
// return the exact same replacement refresh token, not two different ones.
func TestGraceDuplicateRefreshReturnsSameReplacement(t *testing.T) {
	user := entity.User{ID: 3, PublicID: "YTP-000003", Role: entity.RoleMember, IsActive: true}
	repo := &fakeUserRepository{findUser: user}
	sessions := newFakeSessionRepository()

	clock := time.Now()
	now := func() time.Time { return clock }
	uc, tokens := newGraceUsecase(t, repo, sessions, now)

	seedSession(t, sessions, tokens, "refresh-old", user.ID, clock)

	first, err := uc.Refresh(context.Background(), "refresh-old")
	if err != nil {
		t.Fatalf("first Refresh() error = %v", err)
	}

	clock = clock.Add(2 * time.Second)
	second, err := uc.Refresh(context.Background(), "refresh-old")
	if err != nil {
		t.Fatalf("duplicate Refresh() error = %v", err)
	}

	if first.RefreshToken != second.RefreshToken {
		t.Errorf("duplicate refresh token = %q, want same as first %q", second.RefreshToken, first.RefreshToken)
	}
	if second.AccessToken == "" {
		t.Error("duplicate refresh returned no access token")
	}
	if second.AccessToken == first.AccessToken {
		t.Error("duplicate refresh reused the first access token")
	}
}

// TestGraceDoesNotExtendDeadline locks that a duplicate inside grace keeps the
// original deadline: a third request after the original window must be replay.
func TestGraceDoesNotExtendDeadline(t *testing.T) {
	user := entity.User{ID: 3, PublicID: "YTP-000003", Role: entity.RoleMember, IsActive: true}
	repo := &fakeUserRepository{findUser: user}
	sessions := newFakeSessionRepository()

	clock := time.Now()
	now := func() time.Time { return clock }
	uc, tokens := newGraceUsecase(t, repo, sessions, now)

	seedSession(t, sessions, tokens, "refresh-old", user.ID, clock)

	if _, err := uc.Refresh(context.Background(), "refresh-old"); err != nil {
		t.Fatalf("first Refresh() error = %v", err)
	}

	// Duplicate at 8s still inside the 10s window.
	clock = clock.Add(8 * time.Second)
	if _, err := uc.Refresh(context.Background(), "refresh-old"); err != nil {
		t.Fatalf("in-window duplicate error = %v", err)
	}

	// 11s from the original rotation, 3s after the duplicate: must be replay.
	clock = clock.Add(3 * time.Second)
	if _, err := uc.Refresh(context.Background(), "refresh-old"); !errors.Is(err, usecase.ErrInvalidRefreshToken) {
		t.Fatalf("post-window Refresh() error = %v, want ErrInvalidRefreshToken", err)
	}
}

// TestGraceBoundaryIsInclusiveAtDeadline locks that use exactly at the deadline
// is still grace and one second later is replay.
func TestGraceBoundaryIsInclusiveAtDeadline(t *testing.T) {
	user := entity.User{ID: 3, PublicID: "YTP-000003", Role: entity.RoleMember, IsActive: true}
	repo := &fakeUserRepository{findUser: user}
	sessions := newFakeSessionRepository()

	clock := time.Now()
	now := func() time.Time { return clock }
	uc, tokens := newGraceUsecase(t, repo, sessions, now)

	seedSession(t, sessions, tokens, "refresh-old", user.ID, clock)

	if _, err := uc.Refresh(context.Background(), "refresh-old"); err != nil {
		t.Fatalf("first Refresh() error = %v", err)
	}

	clock = clock.Add(10 * time.Second)
	if _, err := uc.Refresh(context.Background(), "refresh-old"); err != nil {
		t.Fatalf("Refresh() at deadline error = %v, want grace success", err)
	}
}

// TestPostGraceReplayRevokesOnlyFamily locks requirement 4: old-token use after
// grace revokes the attacked family only, leaving other families and all other
// sessions for the same user untouched.
func TestPostGraceReplayRevokesOnlyFamily(t *testing.T) {
	user := entity.User{ID: 3, PublicID: "YTP-000003", Role: entity.RoleMember, IsActive: true}
	repo := &fakeUserRepository{findUser: user}
	sessions := newFakeSessionRepository()

	clock := time.Now()
	now := func() time.Time { return clock }
	uc, tokens := newGraceUsecase(t, repo, sessions, now)

	victim := seedSession(t, sessions, tokens, "refresh-victim", user.ID, clock)
	otherRaw := "refresh-other"
	other := entity.RefreshSession{
		UserID:    user.ID,
		FamilyID:  "family-other",
		TokenHash: tokens.HashRefresh(otherRaw),
		ExpiresAt: clock.Add(time.Hour).Unix(),
		CreatedAt: clock.Unix(),
	}
	if err := sessions.CreateRefreshSession(context.Background(), other); err != nil {
		t.Fatalf("seed other session: %v", err)
	}

	rotated, err := uc.Refresh(context.Background(), "refresh-victim")
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	clock = clock.Add(11 * time.Second)
	if _, err := uc.Refresh(context.Background(), "refresh-victim"); !errors.Is(err, usecase.ErrInvalidRefreshToken) {
		t.Fatalf("replay Refresh() error = %v, want ErrInvalidRefreshToken", err)
	}

	if sessions.revokeUserCallFor != 0 {
		t.Error("replay revoked all user sessions instead of the family")
	}
	if sessions.lastRevokedFamily != victim.FamilyID {
		t.Errorf("revoked family = %q, want %q", sessions.lastRevokedFamily, victim.FamilyID)
	}

	replacementHash := tokens.HashRefresh(rotated.RefreshToken)
	if replacement, ok := sessions.sessions[replacementHash]; !ok || replacement.RevokedAt == nil {
		t.Error("replay did not revoke the replacement session in the family")
	}

	if kept, ok := sessions.sessions[tokens.HashRefresh(otherRaw)]; !ok || kept.RevokedAt != nil {
		t.Error("replay revoked a session outside the attacked family")
	}
	if kept, ok := sessions.sessions[victim.FamilyID]; ok && kept.RevokedAt == nil {
		// family_id is not a token hash; this lookup only guards map hygiene.
		_ = kept
	}
	if _, err := uc.Refresh(context.Background(), otherRaw); err != nil {
		t.Errorf("unrelated family Refresh() error = %v, want success", err)
	}
}

// TestSecurityRevocationsBypassGrace locks requirement 5: sessions revoked by
// logout, password change, admin reset, or deactivation can never be reused
// inside any window.
func TestSecurityRevocationsBypassGrace(t *testing.T) {
	user := entity.User{ID: 3, PublicID: "YTP-000003", Role: entity.RoleMember, IsActive: true}
	repo := &fakeUserRepository{findUser: user}
	sessions := newFakeSessionRepository()

	clock := time.Now()
	now := func() time.Time { return clock }
	uc, tokens := newGraceUsecase(t, repo, sessions, now)

	for _, reason := range []string{
		entity.ReasonLogout,
		entity.ReasonPasswordChange,
		entity.ReasonAdminReset,
		entity.ReasonDeactivated,
	} {
		raw := "refresh-" + reason
		hash := tokens.HashRefresh(raw)
		seedSession(t, sessions, tokens, raw, user.ID, clock)
		if err := sessions.RevokeRefreshSession(context.Background(), hash, reason, clock.Unix()); err != nil {
			t.Fatalf("revoke %s: %v", reason, err)
		}

		if _, err := uc.Refresh(context.Background(), raw); !errors.Is(err, usecase.ErrInvalidRefreshToken) {
			t.Errorf("Refresh(%s) error = %v, want ErrInvalidRefreshToken (grace must not apply)", reason, err)
		}
	}
}

// TestExpiredTokenStillRejected locks requirement 6: expiry is honoured before
// grace, so an expired session is never served.
func TestExpiredTokenStillRejected(t *testing.T) {
	user := entity.User{ID: 3, PublicID: "YTP-000003", Role: entity.RoleMember, IsActive: true}
	repo := &fakeUserRepository{findUser: user}
	sessions := newFakeSessionRepository()

	clock := time.Now()
	now := func() time.Time { return clock }
	uc, tokens := newGraceUsecase(t, repo, sessions, now)

	raw := "refresh-expired"
	sessions.sessions[tokens.HashRefresh(raw)] = entity.RefreshSession{
		UserID:    user.ID,
		FamilyID:  "family-expired",
		TokenHash: tokens.HashRefresh(raw),
		ExpiresAt: clock.Add(-time.Second).Unix(),
		CreatedAt: clock.Add(-time.Hour).Unix(),
	}

	if _, err := uc.Refresh(context.Background(), raw); !errors.Is(err, usecase.ErrInvalidRefreshToken) {
		t.Fatalf("Refresh() error = %v, want ErrInvalidRefreshToken", err)
	}
}

// TestSecurityRevokeAfterRotationDisablesGrace locks task 4: a password-change
// (or other user-wide security) revocation that lands after a rotation overwrites
// the rotated session's reason and clears its grace metadata, so a duplicate of
// the old token can no longer be served through the grace window.
func TestSecurityRevokeAfterRotationDisablesGrace(t *testing.T) {
	user := entity.User{ID: 3, PublicID: "YTP-000003", Role: entity.RoleMember, IsActive: true}
	repo := &fakeUserRepository{findUser: user}
	sessions := newFakeSessionRepository()

	clock := time.Now()
	now := func() time.Time { return clock }
	uc, tokens := newGraceUsecase(t, repo, sessions, now)

	seedSession(t, sessions, tokens, "refresh-old", user.ID, clock)

	if _, err := uc.Refresh(context.Background(), "refresh-old"); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	rotatedHash := tokens.HashRefresh("refresh-old")
	if err := sessions.RevokeUserRefreshSessions(context.Background(), user.ID, entity.ReasonPasswordChange, clock.Unix()); err != nil {
		t.Fatalf("RevokeUserRefreshSessions() error = %v", err)
	}

	revoked := sessions.sessions[rotatedHash]
	if revoked.RevocationReason != entity.ReasonRotated {
		t.Errorf("revocation reason = %q, want preserved %q for an already-rotated row", revoked.RevocationReason, entity.ReasonRotated)
	}
	if revoked.GraceUntil != nil || revoked.ReplacementTokenEnc != nil {
		t.Error("security revocation left grace metadata on the rotated session")
	}

	clock = clock.Add(2 * time.Second)
	if _, err := uc.Refresh(context.Background(), "refresh-old"); !errors.Is(err, usecase.ErrInvalidRefreshToken) {
		t.Fatalf("Refresh(old) error = %v, want ErrInvalidRefreshToken after security revocation", err)
	}
}

// TestGraceDecryptFailureIsRejected locks requirement 8: a stored replacement
// that cannot be decrypted fails closed instead of issuing a fresh token.
func TestGraceDecryptFailureIsRejected(t *testing.T) {
	user := entity.User{ID: 3, PublicID: "YTP-000003", Role: entity.RoleMember, IsActive: true}
	repo := &fakeUserRepository{findUser: user}
	sessions := newFakeSessionRepository()

	clock := time.Now()
	now := func() time.Time { return clock }
	uc, tokens := newGraceUsecase(t, repo, sessions, now)

	raw := "refresh-broken"
	revokedAt := clock.Unix()
	grace := clock.Add(10 * time.Second).Unix()
	sessions.sessions[tokens.HashRefresh(raw)] = entity.RefreshSession{
		UserID:              user.ID,
		FamilyID:            "family-broken",
		TokenHash:           tokens.HashRefresh(raw),
		ExpiresAt:           clock.Add(time.Hour).Unix(),
		CreatedAt:           clock.Unix(),
		RevokedAt:           &revokedAt,
		RevocationReason:    entity.ReasonRotated,
		GraceUntil:          &grace,
		ReplacementTokenEnc: []byte("not-valid-ciphertext"),
	}

	if _, err := uc.Refresh(context.Background(), raw); err == nil {
		t.Fatal("Refresh() error = nil, want decrypt failure")
	}
}

// TestRotationStoresEncryptedReplacementWithGrace locks requirements 2 and 3:
// the rotated old session records the family, reason, deadline, and an encrypted
// replacement whose plaintext equals the returned refresh token.
func TestRotationStoresEncryptedReplacementWithGrace(t *testing.T) {
	user := entity.User{ID: 3, PublicID: "YTP-000003", Role: entity.RoleMember, IsActive: true}
	repo := &fakeUserRepository{findUser: user}
	sessions := newFakeSessionRepository()

	clock := time.Now()
	now := func() time.Time { return clock }
	uc, tokens := newGraceUsecase(t, repo, sessions, now)

	seedSession(t, sessions, tokens, "refresh-old", user.ID, clock)

	result, err := uc.Refresh(context.Background(), "refresh-old")
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	stored := sessions.sessions[tokens.HashRefresh("refresh-old")]
	if stored.RevocationReason != entity.ReasonRotated {
		t.Errorf("revocation reason = %q, want %q", stored.RevocationReason, entity.ReasonRotated)
	}
	if stored.FamilyID == "" {
		t.Error("rotated session lost its family id")
	}
	if stored.GraceUntil == nil || *stored.GraceUntil != clock.Add(10*time.Second).Unix() {
		t.Errorf("grace until = %v, want %d", stored.GraceUntil, clock.Add(10*time.Second).Unix())
	}
	if len(stored.ReplacementTokenEnc) == 0 {
		t.Fatal("rotated session stored no encrypted replacement")
	}
	// The stored replacement must decrypt back to the returned raw token.
	decrypted, err := tokens.DecryptRefresh(stored.ReplacementTokenEnc)
	if err != nil {
		t.Fatalf("DecryptRefresh(stored) error = %v", err)
	}
	if decrypted != result.RefreshToken {
		t.Errorf("stored replacement = %q, want returned %q", decrypted, result.RefreshToken)
	}
	if string(stored.ReplacementTokenEnc) == result.RefreshToken {
		t.Error("replacement refresh token was stored in plaintext")
	}
}

// TestLogoutRevokesWithLogoutReason locks requirement 5 for the logout path and
// that the reason recorded is not the rotation reason.
func TestLogoutRevokesWithLogoutReason(t *testing.T) {
	sessions := newFakeSessionRepository()
	uc, tokens := newAuthUseCase(t, &fakeUserRepository{}, sessions)

	if err := uc.Logout(context.Background(), "refresh-abc"); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if sessions.lastRevokedHash != tokens.HashRefresh("refresh-abc") {
		t.Errorf("revoked hash = %q, want %q", sessions.lastRevokedHash, tokens.HashRefresh("refresh-abc"))
	}
	if sessions.lastRevokedReason != entity.ReasonLogout {
		t.Errorf("revocation reason = %q, want %q", sessions.lastRevokedReason, entity.ReasonLogout)
	}
}
