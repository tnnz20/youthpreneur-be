package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
)

type mockUserRepo struct {
	findUserByEmailFunc func(ctx context.Context, email string) (entity.User, error)
	createUserFunc      func(ctx context.Context, user entity.User) (entity.User, error)
}

func (m *mockUserRepo) FindUserByEmail(ctx context.Context, email string) (entity.User, error) {
	if m.findUserByEmailFunc != nil {
		return m.findUserByEmailFunc(ctx, email)
	}
	return entity.User{}, repository.ErrUserNotFound
}

func (m *mockUserRepo) CreateUser(ctx context.Context, user entity.User) (entity.User, error) {
	if m.createUserFunc != nil {
		return m.createUserFunc(ctx, user)
	}
	user.ID = 101
	return user, nil
}

type mockEnterpriseRepo struct {
	createEnterpriseFunc func(ctx context.Context, enterprise entity.Enterprise, event entity.EnterpriseAuditEvent) (entity.Enterprise, error)
	findEnterprisesFunc  func(ctx context.Context, filter entity.EnterpriseFilter) ([]entity.Enterprise, error)
}

func (m *mockEnterpriseRepo) CreateEnterprise(ctx context.Context, enterprise entity.Enterprise, event entity.EnterpriseAuditEvent) (entity.Enterprise, error) {
	if m.createEnterpriseFunc != nil {
		return m.createEnterpriseFunc(ctx, enterprise, event)
	}
	enterprise.ID = 201
	return enterprise, nil
}

func (m *mockEnterpriseRepo) FindEnterprises(ctx context.Context, filter entity.EnterpriseFilter) ([]entity.Enterprise, error) {
	if m.findEnterprisesFunc != nil {
		return m.findEnterprisesFunc(ctx, filter)
	}
	return nil, nil
}

func TestSeedUserEnterprises_Success(t *testing.T) {
	var capturedUser entity.User
	var capturedEnterprise entity.Enterprise
	var capturedAuditEvent entity.EnterpriseAuditEvent

	uRepo := &mockUserRepo{
		createUserFunc: func(ctx context.Context, user entity.User) (entity.User, error) {
			capturedUser = user
			user.ID = 101
			return user, nil
		},
	}
	eRepo := &mockEnterpriseRepo{
		createEnterpriseFunc: func(ctx context.Context, enterprise entity.Enterprise, event entity.EnterpriseAuditEvent) (entity.Enterprise, error) {
			capturedEnterprise = enterprise
			capturedAuditEvent = event
			enterprise.ID = 201
			return enterprise, nil
		},
	}

	records := []UserEnterpriseRecord{
		{
			FullName:             "Jane Doe",
			Email:                "jane@example.com",
			Password:             "password123",
			EnterpriseName:       "Jane Bakery",
			BusinessSector:       entity.BusinessSectorKuliner,
			LegalStatus:          entity.LegalStatusComplete,
			BusinessDigitization: entity.BusinessDigitizationHigh,
			InterventionNeeds:    entity.InterventionNeedsPelatihan,
			TrainingStatus:       entity.ProcessStatusCompleted,
			MentoringStatus:      entity.ProcessStatusOngoing,
			CapitalAccess:        entity.GeneralStatusYes,
			Partnership:          entity.GeneralStatusInProgress,
			InitialTurnover:      "1000000",
			CurrentTurnover:      "2000000",
			District:             "Binuang",
			Status:               entity.EnterpriseStatusActive,
		},
	}

	var buf bytes.Buffer
	fixedNow := func() time.Time { return time.Unix(1700000000, 0) }

	err := seedUserEnterprises(context.Background(), uRepo, eRepo, records, &buf, fixedNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedUser.Profile == nil || capturedUser.Profile.FullName != "JANE DOE" {
		t.Errorf("captured user full_name = %v, want JANE DOE", capturedUser.Profile)
	}
	if capturedEnterprise.EnterpriseName != "JANE BAKERY" {
		t.Errorf("captured enterprise_name = %q, want JANE BAKERY", capturedEnterprise.EnterpriseName)
	}
	if capturedAuditEvent.ChangedFields["enterprise_name"] != "JANE BAKERY" {
		t.Errorf("audit changed fields enterprise_name = %v, want JANE BAKERY", capturedAuditEvent.ChangedFields["enterprise_name"])
	}

	out := buf.String()
	if !strings.Contains(out, "Seeded: user=YTP-") {
		t.Errorf("expected seeded user log, got: %s", out)
	}
	if !strings.Contains(out, "enterprise=TPN-") {
		t.Errorf("expected seeded enterprise log, got: %s", out)
	}
	if !strings.Contains(out, "1 seeded, 0 skipped, 1 total") {
		t.Errorf("expected summary, got: %s", out)
	}
}

func TestSeedUserEnterprises_SkipExistingUser(t *testing.T) {
	uRepo := &mockUserRepo{
		findUserByEmailFunc: func(ctx context.Context, email string) (entity.User, error) {
			return entity.User{
				ID:       55,
				PublicID: "YTP-000055",
				Email:    email,
			}, nil
		},
	}
	eRepo := &mockEnterpriseRepo{
		findEnterprisesFunc: func(ctx context.Context, filter entity.EnterpriseFilter) ([]entity.Enterprise, error) {
			return []entity.Enterprise{{ID: 201, PublicID: "TPN-000001"}}, nil
		},
	}

	records := []UserEnterpriseRecord{
		{
			FullName:       "Existing User",
			Email:          "existing@example.com",
			Password:       "password123",
			EnterpriseName: "Old Shop",
		},
	}

	var buf bytes.Buffer
	fixedNow := func() time.Time { return time.Unix(1700000000, 0) }

	err := seedUserEnterprises(context.Background(), uRepo, eRepo, records, &buf, fixedNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Skipped existing user: existing@example.com") {
		t.Errorf("expected skipped log, got: %s", out)
	}
	if !strings.Contains(out, "0 seeded, 1 skipped, 1 total") {
		t.Errorf("expected summary, got: %s", out)
	}
}

func TestSeedUserEnterprises_ExistingUserMissingEnterpriseSeedsEnterprise(t *testing.T) {
	uRepo := &mockUserRepo{
		findUserByEmailFunc: func(ctx context.Context, email string) (entity.User, error) {
			return entity.User{
				ID:       55,
				PublicID: "YTP-000055",
				Email:    email,
			}, nil
		},
	}
	enterpriseCreated := false
	eRepo := &mockEnterpriseRepo{
		findEnterprisesFunc: func(ctx context.Context, filter entity.EnterpriseFilter) ([]entity.Enterprise, error) {
			return nil, nil // No existing enterprise
		},
		createEnterpriseFunc: func(ctx context.Context, enterprise entity.Enterprise, event entity.EnterpriseAuditEvent) (entity.Enterprise, error) {
			if enterprise.UserID != 55 {
				t.Errorf("expected enterprise UserID 55, got %d", enterprise.UserID)
			}
			enterpriseCreated = true
			enterprise.ID = 201
			enterprise.PublicID = "TPN-000201"
			return enterprise, nil
		},
	}

	records := []UserEnterpriseRecord{
		{
			FullName:       "Existing User Missing Ent",
			Email:          "missingent@example.com",
			Password:       "password123",
			EnterpriseName: "Repaired Shop",
		},
	}

	var buf bytes.Buffer
	fixedNow := func() time.Time { return time.Unix(1700000000, 0) }

	err := seedUserEnterprises(context.Background(), uRepo, eRepo, records, &buf, fixedNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !enterpriseCreated {
		t.Error("expected enterprise to be created for user with missing enterprise")
	}

	out := buf.String()
	if !strings.Contains(out, "1 seeded, 0 skipped, 1 total") {
		t.Errorf("expected 1 seeded, got: %s", out)
	}
}

func TestSeedUserEnterprises_RetryPublicIDCollision(t *testing.T) {
	userCollisionCount := 0
	entCollisionCount := 0

	uRepo := &mockUserRepo{
		createUserFunc: func(ctx context.Context, user entity.User) (entity.User, error) {
			if userCollisionCount < 2 {
				userCollisionCount++
				return entity.User{}, repository.ErrDuplicatePublicID
			}
			user.ID = 101
			return user, nil
		},
	}

	eRepo := &mockEnterpriseRepo{
		createEnterpriseFunc: func(ctx context.Context, enterprise entity.Enterprise, event entity.EnterpriseAuditEvent) (entity.Enterprise, error) {
			if entCollisionCount < 2 {
				entCollisionCount++
				return entity.Enterprise{}, repository.ErrDuplicateEnterprisePublicID
			}
			enterprise.ID = 201
			return enterprise, nil
		},
	}

	records := []UserEnterpriseRecord{
		{
			FullName:       "Retry User",
			Email:          "retry@example.com",
			Password:       "password123",
			EnterpriseName: "Retry Store",
		},
	}

	var buf bytes.Buffer
	fixedNow := func() time.Time { return time.Unix(1700000000, 0) }

	err := seedUserEnterprises(context.Background(), uRepo, eRepo, records, &buf, fixedNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if userCollisionCount != 2 {
		t.Errorf("expected 2 user collisions, got %d", userCollisionCount)
	}
	if entCollisionCount != 2 {
		t.Errorf("expected 2 enterprise collisions, got %d", entCollisionCount)
	}
}

func TestSeedUserEnterprises_DBError(t *testing.T) {
	uRepo := &mockUserRepo{
		createUserFunc: func(ctx context.Context, user entity.User) (entity.User, error) {
			return entity.User{}, errors.New("db connection failure")
		},
	}
	eRepo := &mockEnterpriseRepo{}

	records := []UserEnterpriseRecord{
		{
			FullName:       "Fail User",
			Email:          "fail@example.com",
			Password:       "password123",
			EnterpriseName: "Fail Shop",
		},
	}

	var buf bytes.Buffer
	fixedNow := func() time.Time { return time.Unix(1700000000, 0) }

	err := seedUserEnterprises(context.Background(), uRepo, eRepo, records, &buf, fixedNow)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "db connection failure") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSeedUserEnterprises_UserPublicIDExhaustionReturnsError(t *testing.T) {
	uRepo := &mockUserRepo{
		createUserFunc: func(ctx context.Context, user entity.User) (entity.User, error) {
			return entity.User{}, repository.ErrDuplicatePublicID
		},
	}
	eRepo := &mockEnterpriseRepo{}

	records := []UserEnterpriseRecord{
		{
			FullName:       "Exhaust User",
			Email:          "exhaust@example.com",
			Password:       "password123",
			EnterpriseName: "Exhaust Store",
		},
	}

	var buf bytes.Buffer
	fixedNow := func() time.Time { return time.Unix(1700000000, 0) }

	err := seedUserEnterprises(context.Background(), uRepo, eRepo, records, &buf, fixedNow)
	if err == nil {
		t.Fatal("expected error when public id attempts exhausted, got nil")
	}
	if !strings.Contains(err.Error(), "attempts exhausted") {
		t.Errorf("expected 'attempts exhausted' error, got: %v", err)
	}
}
