package errs

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Code defines standardized domain error codes across the iotp platform.
type Code string

const (
	CodeInvalidArgument     Code = "INVALID_ARGUMENT"
	CodeUnauthorized        Code = "UNAUTHORIZED"
	CodeForbidden           Code = "FORBIDDEN"
	CodeNotFound            Code = "NOT_FOUND"
	CodeConflict            Code = "CONFLICT"
	CodePreconditionFailed  Code = "PRECONDITION_FAILED"
	CodeRateLimited         Code = "RATE_LIMITED"
	CodeInternal            Code = "INTERNAL"
	CodeNotImplemented     Code = "NOT_IMPLEMENTED"
	CodeUnavailable         Code = "UNAVAILABLE"
)

// InvalidParam represents a validation error on a specific field.
type InvalidParam struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// Error is the standard domain error struct conforming to RFC 9457.
type Error struct {
	Code          Code           `json:"code"`
	Message       string         `json:"message"`
	Cause         error          `json:"-"`
	Details       map[string]any `json:"details,omitempty"`
	InvalidParams []InvalidParam `json:"invalid_params,omitempty"`
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// ProblemDetails represents an RFC 9457 Problem Details object.
type ProblemDetails struct {
	Type          string         `json:"type"`
	Title         string         `json:"title"`
	Status        int            `json:"status"`
	Detail        string         `json:"detail"`
	Instance      string         `json:"instance,omitempty"`
	ErrorCode     Code           `json:"error_code"`
	TraceID       string         `json:"trace_id,omitempty"`
	InvalidParams []InvalidParam `json:"invalid_params,omitempty"`
	Details       map[string]any `json:"details,omitempty"`
}

// New creates a new domain Error.
func New(code Code, msg string) *Error {
	return &Error{
		Code:    code,
		Message: msg,
	}
}

// Wrap wraps an existing error with a domain error Code and message.
func Wrap(code Code, msg string, cause error) *Error {
	return &Error{
		Code:    code,
		Message: msg,
		Cause:   cause,
	}
}

// Helper constructors
func InvalidArgument(msg string) *Error {
	return New(CodeInvalidArgument, msg)
}

func InvalidField(fieldName, reason string) *Error {
	return &Error{
		Code:    CodeInvalidArgument,
		Message: fmt.Sprintf("invalid field '%s': %s", fieldName, reason),
		InvalidParams: []InvalidParam{
			{Name: fieldName, Reason: reason},
		},
	}
}

func Unauthorized(msg string) *Error {
	return New(CodeUnauthorized, msg)
}

func Forbidden(msg string) *Error {
	return New(CodeForbidden, msg)
}

func NotFound(msg string) *Error {
	return New(CodeNotFound, msg)
}

func Conflict(msg string) *Error {
	return New(CodeConflict, msg)
}

func PreconditionFailed(msg string) *Error {
	return New(CodePreconditionFailed, msg)
}

func RateLimited(msg string) *Error {
	return New(CodeRateLimited, msg)
}

func Internal(msg string, cause error) *Error {
	return Wrap(CodeInternal, msg, cause)
}

// HTTPStatus returns the corresponding HTTP status code for a domain error code.
func (c Code) HTTPStatus() int {
	switch c {
	case CodeInvalidArgument:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict:
		return http.StatusConflict
	case CodePreconditionFailed:
		return http.StatusPreconditionFailed
	case CodeRateLimited:
		return http.StatusTooManyRequests
	case CodeInternal:
		return http.StatusInternalServerError
	case CodeNotImplemented:
		return http.StatusNotImplemented
	case CodeUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// Title returns an RFC 9457 recommended human-readable title.
func (c Code) Title() string {
	switch c {
	case CodeInvalidArgument:
		return "Bad Request"
	case CodeUnauthorized:
		return "Unauthorized"
	case CodeForbidden:
		return "Forbidden"
	case CodeNotFound:
		return "Not Found"
	case CodeConflict:
		return "Conflict"
	case CodePreconditionFailed:
		return "Precondition Failed"
	case CodeRateLimited:
		return "Too Many Requests"
	case CodeInternal:
		return "Internal Server Error"
	case CodeNotImplemented:
		return "Not Implemented"
	case CodeUnavailable:
		return "Service Unavailable"
	default:
		return "Internal Server Error"
	}
}

// ToProblemDetails converts an error into an RFC 9457 ProblemDetails struct.
func ToProblemDetails(err error, traceID string, instance string) ProblemDetails {
	var domainErr *Error
	if errors.As(err, &domainErr) {
		status := domainErr.Code.HTTPStatus()
		return ProblemDetails{
			Type:          fmt.Sprintf("urn:iotp:error:%s", domainErr.Code),
			Title:         domainErr.Code.Title(),
			Status:        status,
			Detail:        domainErr.Message,
			Instance:      instance,
			ErrorCode:     domainErr.Code,
			TraceID:       traceID,
			InvalidParams: domainErr.InvalidParams,
			Details:       domainErr.Details,
		}
	}

	return ProblemDetails{
		Type:      "urn:iotp:error:INTERNAL",
		Title:     "Internal Server Error",
		Status:    http.StatusInternalServerError,
		Detail:    "An unexpected error occurred",
		Instance:  instance,
		ErrorCode: CodeInternal,
		TraceID:   traceID,
	}
}

// WriteHTTP writes an RFC 9457 application/problem+json response to the given ResponseWriter.
func WriteHTTP(w http.ResponseWriter, err error, traceID, instance string) {
	problem := ToProblemDetails(err, traceID, instance)
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(problem.Status)
	_ = json.NewEncoder(w).Encode(problem)
}
