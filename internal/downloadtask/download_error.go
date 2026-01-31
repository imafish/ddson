package downloadtask

import (
	"fmt"
	"log/slog"
)

// ErrorCode represents error codes as an enum type
// Codes 100-599 are download errors, codes 600+ are gRPC errors
type ErrorCode int

const (
	// Local Errors
	ErrCodeLocalErrorStart ErrorCode = iota + 100
	ErrCodeUnknownDownloadError
	ErrCodeDownloadCancelled
	ErrCodeDownloadInterrupted
	ErrCodePartialDownloadFailed
	ErrCodeResumeFailed
	ErrCodeChecksumMismatch
	ErrCodeFileSizeMismatch
	ErrCodeRangeNotSupported
	ErrCodeInsufficientSpace
	ErrCodeWriteFailed
	ErrCodeReadFailed
	ErrCodeFileCreateFailed
	ErrCodeFileOpenFailed
	ErrCodePermissionDenied
	ErrCodeInvalidPath
	ErrCodeTempDirCreationFailed
	ErrCodeTempFileCleanup

	// Bad Argument
	ErrCodeInvalidURL
	ErrCodeUnsupportedProtocol
	ErrCodeHostUnreachable
	ErrCodeDNSLookupFailed
	ErrCodeDownloadTaskNotFound

	// Connection errors (2xx)
	ErrCodeRemoteErrorStart ErrorCode = iota + 500
	ErrCodeHTTPError
	ErrCodeUnexpectedStatus

	// gRPC Connection errors
	ErrCodeGRPCConnectionFailed ErrorCode = iota + 600
	ErrCodeGRPCConnectionClosed
	ErrCodeGRPCConnectionTimeout
	ErrCodeGRPCUnavailable

	// gRPC Authentication/Authorization errors
	ErrCodeGRPCUnauthenticated ErrorCode = iota + 700
	ErrCodeGRPCPermissionDenied
	ErrCodeGRPCInvalidCredentials

	// gRPC Request/Response errors
	ErrCodeGRPCInvalidRequest ErrorCode = iota + 800
	ErrCodeGRPCInvalidResponse
	ErrCodeGRPCDeadlineExceeded
	ErrCodeGRPCCancelled
	ErrCodeGRPCResourceExhausted

	// gRPC Server errors (9xx)
	ErrCodeGRPCInternal ErrorCode = iota + 900
	ErrCodeGRPCUnimplemented
	ErrCodeGRPCDataLoss

	// Stream errors (10xx)
	ErrCodeStreamClosed ErrorCode = iota + 1000
	ErrCodeStreamSendFailed
	ErrCodeStreamRecvFailed

	// Task errors (11xx)
	ErrCodeTaskNotFound ErrorCode = iota + 1100
	ErrCodeTaskAlreadyExists
	ErrCodeTaskCancelled
	ErrCodeTaskFailed
	ErrCodeSubtaskFailed

	// Agent communication errors (12xx)
	ErrCodeAgentNotFound ErrorCode = iota + 1200
	ErrCodeAgentNotResponding
	ErrCodeAgentDisconnected
	ErrCodeAgentBusy
	ErrCodeNoAgentsAvailable
)

// errorMessages maps error codes to their human-readable messages
var errorMessages = map[ErrorCode]string{
	// Local errors
	ErrCodeInsufficientSpace:     "insufficient disk space",
	ErrCodeWriteFailed:           "failed to write to file",
	ErrCodeReadFailed:            "failed to read from file",
	ErrCodeFileCreateFailed:      "failed to create file",
	ErrCodeFileOpenFailed:        "failed to open file",
	ErrCodePermissionDenied:      "permission denied",
	ErrCodeInvalidPath:           "invalid file path",
	ErrCodeTempFileCleanup:       "failed to clean up temporary files",
	ErrCodeTempDirCreationFailed: "failed to create temporary directory",

	ErrCodeUnknownDownloadError:  "unknown download error",
	ErrCodeDownloadCancelled:     "download cancelled by user",
	ErrCodeDownloadInterrupted:   "download interrupted",
	ErrCodePartialDownloadFailed: "partial download failed",
	ErrCodeResumeFailed:          "failed to resume download",
	ErrCodeChecksumMismatch:      "checksum verification failed",
	ErrCodeFileSizeMismatch:      "downloaded file size does not match expected size",
	ErrCodeRangeNotSupported:     "server does not support partial content (range requests)",

	// Remote errors
	ErrCodeHTTPError:        "HTTP error occurred",
	ErrCodeUnexpectedStatus: "unexpected download status reported by agent",

	// Bad Argument
	ErrCodeInvalidURL:          "invalid URL",
	ErrCodeUnsupportedProtocol: "unsupported protocol",
	ErrCodeHostUnreachable:     "host unreachable",
	ErrCodeDNSLookupFailed:     "DNS lookup failed",

	// gRPC Connection errors
	ErrCodeGRPCConnectionFailed:  "failed to connect to gRPC server",
	ErrCodeGRPCConnectionClosed:  "gRPC connection closed unexpectedly",
	ErrCodeGRPCConnectionTimeout: "gRPC connection timed out",
	ErrCodeGRPCUnavailable:       "gRPC service unavailable",

	// gRPC Authentication/Authorization errors
	ErrCodeGRPCUnauthenticated:    "gRPC call unauthenticated",
	ErrCodeGRPCPermissionDenied:   "gRPC permission denied",
	ErrCodeGRPCInvalidCredentials: "invalid gRPC credentials",

	// gRPC Request/Response errors
	ErrCodeGRPCInvalidRequest:    "invalid gRPC request",
	ErrCodeGRPCInvalidResponse:   "invalid gRPC response",
	ErrCodeGRPCDeadlineExceeded:  "gRPC deadline exceeded",
	ErrCodeGRPCCancelled:         "gRPC request cancelled",
	ErrCodeGRPCResourceExhausted: "gRPC resource exhausted",

	// gRPC Server errors
	ErrCodeGRPCInternal:      "internal gRPC server error",
	ErrCodeGRPCUnimplemented: "gRPC method not implemented",
	ErrCodeGRPCDataLoss:      "gRPC unrecoverable data loss",

	// Agent communication errors
	ErrCodeAgentNotFound:      "agent not found",
	ErrCodeAgentNotResponding: "agent not responding",
	ErrCodeAgentDisconnected:  "agent disconnected",
	ErrCodeAgentBusy:          "agent is busy",
	ErrCodeNoAgentsAvailable:  "no agents available",

	// Task errors
	ErrCodeTaskNotFound:      "task not found",
	ErrCodeTaskAlreadyExists: "task already exists",
	ErrCodeTaskCancelled:     "task was cancelled",
	ErrCodeTaskFailed:        "task execution failed",
	ErrCodeSubtaskFailed:     "subtask execution failed",

	// Stream errors
	ErrCodeStreamClosed:     "gRPC stream closed",
	ErrCodeStreamSendFailed: "failed to send on gRPC stream",
	ErrCodeStreamRecvFailed: "failed to receive from gRPC stream",
}

// String returns the human-readable message for an ErrorCode
func (e ErrorCode) String() string {
	if msg, ok := errorMessages[e]; ok {
		return msg
	}
	return fmt.Sprintf("unknown error code: %d", e)
}

// IsGrpcError returns true if this error code is a gRPC-related error (600+)
func (e ErrorCode) IsGrpcError() bool {
	return e >= 600
}

// IsDownloadError returns true if this error code is a download-related error (100-599)
func (e ErrorCode) IsDownloadError() bool {
	return e >= 100 && e < 600
}

// TODO: Download error should have a field to link it to a task, because it is always from a download task
// DownloadError wraps an error with additional context about the download or gRPC operation
type DownloadError struct {
	Code    ErrorCode
	Cause   error
	Message string

	HTTPCode int // for http errors

	Method  string // gRPC method name (for gRPC errors)
	AgentID int    // Agent ID (for gRPC errors)
}

func (e *DownloadError) Error() string {
	if e.Code.IsGrpcError() {
		if e.AgentID != -1 {
			if e.Cause != nil {
				return fmt.Sprintf("gRPC error [%d] [%s] for agent %s: %s - %v", e.Code, e.Method, e.AgentID, e.Message, e.Cause)
			}
			return fmt.Sprintf("gRPC error [%d] [%s] for agent %s: %s", e.Code, e.Method, e.AgentID, e.Message)
		}
		if e.Cause != nil {
			return fmt.Sprintf("gRPC error [%d] [%s]: %s - %v", e.Code, e.Method, e.Message, e.Cause)
		}
		return fmt.Sprintf("gRPC error [%d] [%s]: %s", e.Code, e.Method, e.Message)
	}

	if e.HTTPCode != 0 {
		return fmt.Sprintf("download error [%d] (HTTP %d): %s - %v", e.Code, e.HTTPCode, e.Message, e.Cause)
	}
	if e.Cause != nil {
		return fmt.Sprintf("download error [%d]: %s - %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("download error [%d]: %s", e.Code, e.Message)
}

func (e *DownloadError) Unwrap() error {
	return e.Cause
}

// IsGrpcError returns true if this is a gRPC-related error
func (e *DownloadError) IsGrpcError() bool {
	return e.Code.IsGrpcError()
}

// IsDownloadError returns true if this is a download-related error
func (e *DownloadError) IsDownloadError() bool {
	return e.Code.IsDownloadError()
}

// IsRetryable returns true if the error is potentially retryable
// If not retryable, the error is considered fatal
func (e *DownloadError) IsRetryable() bool {
	switch e.Code {
	case ErrCodeHTTPError:
		switch e.HTTPCode {
		case 400, 401, 403, 404, 405, 411, 412, 413, 414, 415, 416, 417, 422, 426, 428, 429, 431, 451:
			return true
		}
		return false

		// gRPC retryable errors
	case ErrCodeGRPCConnectionTimeout,
		ErrCodeGRPCUnavailable,
		ErrCodeGRPCDeadlineExceeded,
		ErrCodeGRPCResourceExhausted,
		ErrCodeAgentNotResponding,
		ErrCodeAgentBusy:
		return true
	}
	return false
}

// analyze the error message and extract HTTP related issues from it.
// TODO: This is ill-formed. Should return error in response. Should not use grpc error mechanism for this.
func NewDownloadErrorFromError(code ErrorCode, cause error, agentID int, method string) *DownloadError {
	httpCode := extractHTTPStatusCode(cause)
	if httpCode != 0 {
		return NewDownloadErrorWithHTTPCode(ErrCodeHTTPError, cause, httpCode)
	}

	return &DownloadError{
		Code:    code,
		Cause:   cause,
		AgentID: agentID,
		Method:  method,
	}
}

// NewDownloadError creates a new DownloadError with an error code
func NewDownloadError(code ErrorCode, cause error) *DownloadError {
	return &DownloadError{
		Code:    code,
		Cause:   cause,
		AgentID: -1,
	}
}

// NewDownloadErrorWithMessage creates a new DownloadError with a custom message
func NewDownloadErrorWithMessage(code ErrorCode, message string) *DownloadError {
	return &DownloadError{
		Code:    code,
		Message: message,
		AgentID: -1,
	}
}

// NewDownloadErrorWithHTTPCode creates a new DownloadError with an HTTP status code
func NewDownloadErrorWithHTTPCode(code ErrorCode, cause error, httpCode int) *DownloadError {
	return &DownloadError{
		Code:     code,
		Cause:    cause,
		HTTPCode: httpCode,
		AgentID:  -1,
	}
}

// NewGRPCError creates a new DownloadError for gRPC errors with agent context
func NewGRPCError(code ErrorCode, cause error, agentID int, method string) *DownloadError {
	return &DownloadError{
		Code:    code,
		Cause:   cause,
		Method:  method,
		AgentID: agentID,
	}
}

func extractHTTPStatusCode(err error) int {
	if err == nil {
		return 0
	}

	slog.Debug("Extracting HTTP status code from error", "error", err)

	// Look for "unexpected HTTP status: <code> ..." or "unexpected HTTP status: <code>"
	msg := err.Error()
	var code int

	// Try to match "unexpected HTTP status: <code>"
	n, _ := fmt.Sscanf(msg, "unexpected HTTP status: %d", &code)
	if n >= 1 {
		return code
	}

	return 0
}
