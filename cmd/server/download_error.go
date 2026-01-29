package main

import "fmt"

// ErrorCode represents error codes as an enum type
// Codes 100-599 are download errors, codes 600+ are gRPC errors
type ErrorCode int

const (
	// Connection errors (1xx)
	ErrCodeHTTPError ErrorCode = iota + 100

	ErrCodeUnknownDownloadError ErrorCode = iota + 200
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
	ErrCodeTempFileCleanup

	// URL errors (5xx)
	ErrCodeInvalidURL ErrorCode = iota + 500
	ErrCodeUnsupportedProtocol
	ErrCodeHostUnreachable
	ErrCodeDNSLookupFailed

	// gRPC Connection errors (6xx)
	ErrCodeGRPCConnectionFailed ErrorCode = iota + 600
	ErrCodeGRPCConnectionClosed
	ErrCodeGRPCConnectionTimeout
	ErrCodeGRPCUnavailable

	// gRPC Authentication/Authorization errors (7xx)
	ErrCodeGRPCUnauthenticated ErrorCode = iota + 700
	ErrCodeGRPCPermissionDenied
	ErrCodeGRPCInvalidCredentials

	// gRPC Request/Response errors (8xx)
	ErrCodeGRPCInvalidRequest ErrorCode = iota + 800
	ErrCodeGRPCInvalidResponse
	ErrCodeGRPCDeadlineExceeded
	ErrCodeGRPCCancelled
	ErrCodeGRPCResourceExhausted

	// gRPC Server errors (9xx)
	ErrCodeGRPCInternal ErrorCode = iota + 900
	ErrCodeGRPCUnimplemented
	ErrCodeGRPCDataLoss

	// Agent communication errors (10xx)
	ErrCodeAgentNotFound ErrorCode = iota + 1000
	ErrCodeAgentNotResponding
	ErrCodeAgentDisconnected
	ErrCodeAgentBusy
	ErrCodeNoAgentsAvailable

	// Task errors (11xx)
	ErrCodeTaskNotFound ErrorCode = iota + 1100
	ErrCodeTaskAlreadyExists
	ErrCodeTaskCancelled
	ErrCodeTaskFailed
	ErrCodeSubtaskFailed

	// Stream errors (12xx)
	ErrCodeStreamClosed ErrorCode = iota + 1200
	ErrCodeStreamSendFailed
	ErrCodeStreamRecvFailed
)

// errorMessages maps error codes to their human-readable messages
var errorMessages = map[ErrorCode]string{
	ErrCodeHTTPError:             "HTTP error occurred",
	ErrCodeUnknownDownloadError:  "unknown download error",
	ErrCodeDownloadCancelled:     "download cancelled by user",
	ErrCodeDownloadInterrupted:   "download interrupted",
	ErrCodePartialDownloadFailed: "partial download failed",
	ErrCodeResumeFailed:          "failed to resume download",
	ErrCodeChecksumMismatch:      "checksum verification failed",
	ErrCodeFileSizeMismatch:      "downloaded file size does not match expected size",
	ErrCodeRangeNotSupported:     "server does not support partial content (range requests)",

	// File system errors
	ErrCodeInsufficientSpace: "insufficient disk space",
	ErrCodeWriteFailed:       "failed to write to file",
	ErrCodeReadFailed:        "failed to read from file",
	ErrCodeFileCreateFailed:  "failed to create file",
	ErrCodeFileOpenFailed:    "failed to open file",
	ErrCodePermissionDenied:  "permission denied",
	ErrCodeInvalidPath:       "invalid file path",
	ErrCodeTempFileCleanup:   "failed to clean up temporary files",

	// URL errors
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

// DownloadError wraps an error with additional context about the download or gRPC operation
type DownloadError struct {
	URL      string
	Code     ErrorCode
	Cause    error
	HTTPCode int
	Method   string // gRPC method name (for gRPC errors)
	AgentID  string // Agent ID (for gRPC errors)
	Message  string
}

func (e *DownloadError) Error() string {
	if e.Code.IsGrpcError() {
		if e.AgentID != "" {
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
		return fmt.Sprintf("download error [%d] for %s (HTTP %d): %s - %v", e.Code, e.URL, e.HTTPCode, e.Message, e.Cause)
	}
	if e.Cause != nil {
		return fmt.Sprintf("download error [%d] for %s: %s - %v", e.Code, e.URL, e.Message, e.Cause)
	}
	return fmt.Sprintf("download error [%d] for %s: %s", e.Code, e.URL, e.Message)
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

// NewDownloadError creates a new DownloadError with an error code
func NewDownloadError(url string, code ErrorCode, cause error) *DownloadError {
	return &DownloadError{
		URL:     url,
		Code:    code,
		Cause:   cause,
		Message: code.String(),
	}
}

// NewDownloadErrorWithMessage creates a new DownloadError with a custom message
func NewDownloadErrorWithMessage(url string, code ErrorCode, message string) *DownloadError {
	return &DownloadError{
		URL:     url,
		Code:    code,
		Message: message,
	}
}

// NewDownloadErrorWithHTTPCode creates a new DownloadError with an HTTP status code
func NewDownloadErrorWithHTTPCode(url string, code ErrorCode, cause error, httpCode int) *DownloadError {
	return &DownloadError{
		URL:      url,
		Code:     code,
		Cause:    cause,
		HTTPCode: httpCode,
		Message:  code.String(),
	}
}

// NewGRPCError creates a new DownloadError for gRPC errors with agent context
func NewGRPCError(method string, agentID string, code ErrorCode, cause error) *DownloadError {
	return &DownloadError{
		Code:    code,
		Method:  method,
		AgentID: agentID,
		Cause:   cause,
		Message: code.String(),
	}
}
