package downloadtask

import (
	"fmt"
	"testing"

	"github.com/imafish/ddson/internal/pb"
)

func TestDownloadChunkHandlesErrorStatus(t *testing.T) {
	// Test different error scenarios to ensure proto conversion works
	tests := []struct {
		name          string
		errorCode     ErrorCode
		errorMessage  string
		httpCode      int
		expectErrCode ErrorCode
	}{
		{
			name:          "HTTP error with code",
			errorCode:     ErrCodeHTTPError,
			errorMessage:  "Not Found",
			httpCode:      404,
			expectErrCode: ErrCodeHTTPError,
		},
		{
			name:          "File permission error",
			errorCode:     ErrCodePermissionDenied,
			errorMessage:  "Permission denied",
			httpCode:      0,
			expectErrCode: ErrCodePermissionDenied,
		},
		{
			name:          "Connection error",
			errorCode:     ErrCodeHostUnreachable,
			errorMessage:  "Host unreachable",
			httpCode:      0,
			expectErrCode: ErrCodeHostUnreachable,
		},
		{
			name:          "Checksum mismatch",
			errorCode:     ErrCodeChecksumMismatch,
			errorMessage:  "Checksum verification failed",
			httpCode:      0,
			expectErrCode: ErrCodeChecksumMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create error to send
			downloadErr := &DownloadError{
				Code:     tt.errorCode,
				Message:  tt.errorMessage,
				HTTPCode: tt.httpCode,
				AgentID:  -1,
			}

			// Create mock response with ERROR status
			response := &pb.DownloadStatus{
				Status: pb.DownloadStatusType_ERROR,
				Error:  downloadErr.ToProto(),
			}

			// Verify the error can be converted correctly
			if response.Error == nil {
				t.Fatal("Expected error to be set in response")
			}

			convertedErr := FromProto(response.Error)

			if convertedErr.Code != tt.expectErrCode {
				t.Errorf("Expected error code %d, got %d", tt.expectErrCode, convertedErr.Code)
			}

			if tt.httpCode != 0 && convertedErr.HTTPCode != tt.httpCode {
				t.Errorf("Expected HTTP code %d, got %d", tt.httpCode, convertedErr.HTTPCode)
			}

			if convertedErr.Message != tt.errorMessage {
				t.Errorf("Expected message '%s', got '%s'", tt.errorMessage, convertedErr.Message)
			}
		})
	}
}

func TestDownloadChunkHandlesErrorWithoutDetails(t *testing.T) {
	// Test that we handle ERROR status without error details gracefully
	downloadErr := FromProto(&pb.DownloadError{
		Code:    int32(ErrCodeSubtaskFailed),
		Message: "Generic error message",
	})

	if downloadErr == nil {
		t.Fatal("Expected non-nil error")
	}

	if downloadErr.Code != ErrCodeSubtaskFailed {
		t.Errorf("Expected error code %d, got %d", ErrCodeSubtaskFailed, downloadErr.Code)
	}

	if downloadErr.Message != "Generic error message" {
		t.Errorf("Expected message 'Generic error message', got '%s'", downloadErr.Message)
	}
}

func TestErrorStatusPropagation(t *testing.T) {
	// Test that errors are properly propagated through the conversion chain
	tests := []struct {
		name       string
		createErr  func() *DownloadError
		checkAfter func(*testing.T, *DownloadError)
	}{
		{
			name: "HTTP 500 error",
			createErr: func() *DownloadError {
				return NewDownloadErrorWithHTTPCode(
					ErrCodeHTTPError,
					fmt.Errorf("server error"),
					500,
				)
			},
			checkAfter: func(t *testing.T, err *DownloadError) {
				if err.HTTPCode != 500 {
					t.Errorf("Expected HTTP code 500, got %d", err.HTTPCode)
				}
				if !err.IsDownloadError() {
					t.Error("Expected download error")
				}
			},
		},
		{
			name: "gRPC connection error",
			createErr: func() *DownloadError {
				return NewGRPCError(
					ErrCodeGRPCConnectionFailed,
					fmt.Errorf("connection refused"),
					42,
					"DownloadPart",
				)
			},
			checkAfter: func(t *testing.T, err *DownloadError) {
				if err.AgentID != 42 {
					t.Errorf("Expected agent ID 42, got %d", err.AgentID)
				}
				if err.Method != "DownloadPart" {
					t.Errorf("Expected method 'DownloadPart', got '%s'", err.Method)
				}
				if !err.IsGrpcError() {
					t.Error("Expected gRPC error")
				}
			},
		},
		{
			name: "retryable error",
			createErr: func() *DownloadError {
				return NewDownloadErrorWithHTTPCode(
					ErrCodeHTTPError,
					fmt.Errorf("too many requests"),
					429,
				)
			},
			checkAfter: func(t *testing.T, err *DownloadError) {
				if !err.IsRetryable() {
					t.Error("Expected error to be retryable")
				}
			},
		},
		{
			name: "non-retryable error",
			createErr: func() *DownloadError {
				return NewDownloadErrorWithMessage(
					ErrCodeChecksumMismatch,
					"checksum failed",
				)
			},
			checkAfter: func(t *testing.T, err *DownloadError) {
				if err.IsRetryable() {
					t.Error("Expected error to not be retryable")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create error
			originalErr := tt.createErr()

			// Convert to proto and back
			pbErr := originalErr.ToProto()
			convertedErr := FromProto(pbErr)

			// Verify properties are preserved
			if convertedErr.Code != originalErr.Code {
				t.Errorf("Code mismatch: expected %d, got %d", originalErr.Code, convertedErr.Code)
			}

			// Run additional checks
			tt.checkAfter(t, convertedErr)
		})
	}
}
