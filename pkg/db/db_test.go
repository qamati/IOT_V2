package db_test

import (
	"context"
	"testing"

	"github.com/qamati/IOT_V2/pkg/db"
	"github.com/stretchr/testify/assert"
)

func TestConnString(t *testing.T) {
	cfg := db.Config{
		Host:     "localhost",
		Port:     5432,
		Database: "iotp_test",
		User:     "iotp_user",
		Password: "secretpassword",
		SSLMode:  "disable",
	}

	expected := "postgres://iotp_user:secretpassword@localhost:5432/iotp_test?sslmode=disable"
	assert.Equal(t, expected, cfg.ConnString())
}

func TestTenantContext(t *testing.T) {
	ctx := context.Background()
	assert.Empty(t, db.TenantID(ctx))

	ctxWithTenant := db.WithTenant(ctx, "018f2d5e-5b12-7890-8912-34567890abcd")
	assert.Equal(t, "018f2d5e-5b12-7890-8912-34567890abcd", db.TenantID(ctxWithTenant))
}
