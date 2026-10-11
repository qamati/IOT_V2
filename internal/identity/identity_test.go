package identity_test

import (
	"context"
	"testing"
	"time"

	"github.com/qamati/IOT_V2/internal/identity"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArgon2idPasswordHashing(t *testing.T) {
	password := "SecretPass123!"

	hash, err := identity.HashPassword(password)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)

	assert.True(t, identity.VerifyPassword(password, hash))
	assert.False(t, identity.VerifyPassword("WrongPassword", hash))
}

func TestLoginAndJWTFlow(t *testing.T) {
	store := identity.NewMemoryUserStore()
	svc := identity.NewService(store, []byte("test-jwt-secret-key-32-bytes!!"), 1*time.Hour)

	ctx := context.Background()
	tenantID := ids.MustNew()
	user := &model.User{
		ID:          ids.MustNew(),
		CreatedTime: time.Now().UnixMilli(),
		TenantID:    &tenantID,
		Authority:   model.AuthorityTenantAdmin,
		Email:       "tenant@iotp.local",
		FirstName:   "Tenant",
		LastName:    "Admin",
		Version:     1,
	}

	err := svc.Register(ctx, user, "CorrectPassword456!")
	require.NoError(t, err)

	// 1. Successful login
	token, loggedUser, err := svc.Login(ctx, "tenant@iotp.local", "CorrectPassword456!")
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, user.ID, loggedUser.ID)

	// 2. Validate token
	claims, err := svc.ValidateAccessToken(token)
	require.NoError(t, err)
	assert.Equal(t, user.ID, claims.UserID)
	assert.Equal(t, tenantID.String(), claims.TenantID)
	assert.Equal(t, model.AuthorityTenantAdmin, claims.Authority)

	// 3. Failed login with wrong password
	_, _, err = svc.Login(ctx, "tenant@iotp.local", "WrongPassword")
	assert.Error(t, err)

	// 4. Failed login with non-existent user
	_, _, err = svc.Login(ctx, "unknown@iotp.local", "SomePassword")
	assert.Error(t, err)
}
