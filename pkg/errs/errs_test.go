package errs_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/qamati/IOT_V2/pkg/errs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDomainErrors(t *testing.T) {
	orig := errors.New("underlying db error")
	err := errs.Wrap(errs.CodeNotFound, "device not found", orig)

	assert.Equal(t, "[NOT_FOUND] device not found: underlying db error", err.Error())
	assert.True(t, errors.Is(err, orig))

	var domainErr *errs.Error
	require.True(t, errors.As(err, &domainErr))
	assert.Equal(t, errs.CodeNotFound, domainErr.Code)
	assert.Equal(t, http.StatusNotFound, domainErr.Code.HTTPStatus())
}

func TestProblemDetails(t *testing.T) {
	err := errs.InvalidField("name", "cannot be blank")
	pd := errs.ToProblemDetails(err, "trace-12345", "/api/v1/devices")

	assert.Equal(t, http.StatusBadRequest, pd.Status)
	assert.Equal(t, "urn:iotp:error:INVALID_ARGUMENT", pd.Type)
	assert.Equal(t, "Bad Request", pd.Title)
	assert.Equal(t, "trace-12345", pd.TraceID)
	assert.Equal(t, "/api/v1/devices", pd.Instance)
	require.Len(t, pd.InvalidParams, 1)
	assert.Equal(t, "name", pd.InvalidParams[0].Name)
	assert.Equal(t, "cannot be blank", pd.InvalidParams[0].Reason)
}

func TestWriteHTTP(t *testing.T) {
	rec := httptest.NewRecorder()
	err := errs.Unauthorized("token expired")

	errs.WriteHTTP(rec, err, "req-xyz", "/auth/login")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))

	var body errs.ProblemDetails
	errDecode := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, errDecode)
	assert.Equal(t, errs.CodeUnauthorized, body.ErrorCode)
	assert.Equal(t, "token expired", body.Detail)
	assert.Equal(t, "req-xyz", body.TraceID)
}

func TestUnknownErrorFallback(t *testing.T) {
	plainErr := errors.New("unexpected nil pointer")
	pd := errs.ToProblemDetails(plainErr, "trace-999", "/test")

	assert.Equal(t, http.StatusInternalServerError, pd.Status)
	assert.Equal(t, errs.CodeInternal, pd.ErrorCode)
	assert.Equal(t, "An unexpected error occurred", pd.Detail)
}
