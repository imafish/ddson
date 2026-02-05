package downloadtask

import (
	"errors"
	"testing"
)

func TestErrorCodeStringUnknown(t *testing.T) {
	msg := ErrorCode(9999).String()
	if msg != "unknown error code: 9999" {
		t.Fatalf("unexpected message: %s", msg)
	}
}

func TestIsRetryable(t *testing.T) {
	httpErr := NewDownloadErrorWithHTTPCode(ErrCodeHTTPError, errors.New("unexpected HTTP status: 404"), 404)
	if !httpErr.IsRetryable() {
		t.Fatalf("expected HTTP 404 to be retryable")
	}

	httpErr = NewDownloadErrorWithHTTPCode(ErrCodeHTTPError, errors.New("unexpected HTTP status: 500"), 500)
	if httpErr.IsRetryable() {
		t.Fatalf("expected HTTP 500 to be non-retryable")
	}

	grpcErr := NewGRPCError(ErrCodeGRPCUnavailable, errors.New("unavailable"), 1, "DownloadPart")
	if !grpcErr.IsRetryable() {
		t.Fatalf("expected gRPC unavailable to be retryable")
	}
}

func TestExtractHTTPStatusCode(t *testing.T) {
	code := extractHTTPStatusCode(errors.New("unexpected HTTP status: 416"))
	if code != 416 {
		t.Fatalf("expected 416, got %d", code)
	}

	code = extractHTTPStatusCode(errors.New("some other error"))
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
}
