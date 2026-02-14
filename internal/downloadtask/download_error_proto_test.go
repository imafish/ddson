package downloadtask

import (
	"fmt"
	"testing"

	"github.com/imafish/ddson/internal/pb"
)

func TestDownloadErrorToProto(t *testing.T) {
	tests := []struct {
		name     string
		err      *DownloadError
		validate func(*testing.T, *pb.DownloadError)
	}{
		{
			name: "nil error",
			err:  nil,
			validate: func(t *testing.T, pbErr *pb.DownloadError) {
				if pbErr != nil {
					t.Errorf("Expected nil, got %v", pbErr)
				}
			},
		},
		{
			name: "simple error with code",
			err: &DownloadError{
				Code:    ErrCodeFileCreateFailed,
				Message: "custom message",
				AgentID: -1,
			},
			validate: func(t *testing.T, pbErr *pb.DownloadError) {
				if pbErr == nil {
					t.Fatal("Expected non-nil protobuf error")
				}
				if pbErr.Code != int32(ErrCodeFileCreateFailed) {
					t.Errorf("Expected code %d, got %d", ErrCodeFileCreateFailed, pbErr.Code)
				}
				if pbErr.Message != "custom message" {
					t.Errorf("Expected message 'custom message', got '%s'", pbErr.Message)
				}
				if pbErr.Cause != "" {
					t.Errorf("Expected empty cause, got '%s'", pbErr.Cause)
				}
			},
		},
		{
			name: "error with cause",
			err: &DownloadError{
				Code:    ErrCodeHTTPError,
				Message: "HTTP error",
				Cause:   fmt.Errorf("connection timeout"),
				AgentID: -1,
			},
			validate: func(t *testing.T, pbErr *pb.DownloadError) {
				if pbErr == nil {
					t.Fatal("Expected non-nil protobuf error")
				}
				if pbErr.Code != int32(ErrCodeHTTPError) {
					t.Errorf("Expected code %d, got %d", ErrCodeHTTPError, pbErr.Code)
				}
				if pbErr.Cause != "connection timeout" {
					t.Errorf("Expected cause 'connection timeout', got '%s'", pbErr.Cause)
				}
			},
		},
		{
			name: "error with HTTP code",
			err: &DownloadError{
				Code:     ErrCodeHTTPError,
				HTTPCode: 404,
				AgentID:  -1,
			},
			validate: func(t *testing.T, pbErr *pb.DownloadError) {
				if pbErr == nil {
					t.Fatal("Expected non-nil protobuf error")
				}
				if pbErr.HttpCode != 404 {
					t.Errorf("Expected HTTP code 404, got %d", pbErr.HttpCode)
				}
			},
		},
		{
			name: "gRPC error with agent info",
			err: &DownloadError{
				Code:    ErrCodeGRPCConnectionFailed,
				Method:  "DownloadPart",
				AgentID: 42,
			},
			validate: func(t *testing.T, pbErr *pb.DownloadError) {
				if pbErr == nil {
					t.Fatal("Expected non-nil protobuf error")
				}
				if pbErr.Method != "DownloadPart" {
					t.Errorf("Expected method 'DownloadPart', got '%s'", pbErr.Method)
				}
				if pbErr.AgentId != 42 {
					t.Errorf("Expected agent ID 42, got %d", pbErr.AgentId)
				}
			},
		},
		{
			name: "error without custom message uses code string",
			err: &DownloadError{
				Code:    ErrCodeChecksumMismatch,
				AgentID: -1,
			},
			validate: func(t *testing.T, pbErr *pb.DownloadError) {
				if pbErr == nil {
					t.Fatal("Expected non-nil protobuf error")
				}
				if pbErr.Message == "" {
					t.Errorf("Expected message to be populated from error code, got empty")
				}
				if pbErr.Message != ErrCodeChecksumMismatch.String() {
					t.Errorf("Expected message '%s', got '%s'", ErrCodeChecksumMismatch.String(), pbErr.Message)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pbErr := tt.err.ToProto()
			tt.validate(t, pbErr)
		})
	}
}

func TestFromProto(t *testing.T) {
	tests := []struct {
		name     string
		pbErr    *pb.DownloadError
		validate func(*testing.T, *DownloadError)
	}{
		{
			name:  "nil proto error",
			pbErr: nil,
			validate: func(t *testing.T, err *DownloadError) {
				if err != nil {
					t.Errorf("Expected nil, got %v", err)
				}
			},
		},
		{
			name: "simple proto error",
			pbErr: &pb.DownloadError{
				Code:    int32(ErrCodeFileCreateFailed),
				Message: "test message",
			},
			validate: func(t *testing.T, err *DownloadError) {
				if err == nil {
					t.Fatal("Expected non-nil error")
				}
				if err.Code != ErrCodeFileCreateFailed {
					t.Errorf("Expected code %d, got %d", ErrCodeFileCreateFailed, err.Code)
				}
				if err.Message != "test message" {
					t.Errorf("Expected message 'test message', got '%s'", err.Message)
				}
			},
		},
		{
			name: "proto error with cause",
			pbErr: &pb.DownloadError{
				Code:    int32(ErrCodeHTTPError),
				Message: "HTTP error",
				Cause:   "timeout occurred",
			},
			validate: func(t *testing.T, err *DownloadError) {
				if err == nil {
					t.Fatal("Expected non-nil error")
				}
				if err.Cause == nil {
					t.Fatal("Expected non-nil cause")
				}
				if err.Cause.Error() != "timeout occurred" {
					t.Errorf("Expected cause 'timeout occurred', got '%s'", err.Cause.Error())
				}
			},
		},
		{
			name: "proto error with HTTP code",
			pbErr: &pb.DownloadError{
				Code:     int32(ErrCodeHTTPError),
				HttpCode: 503,
			},
			validate: func(t *testing.T, err *DownloadError) {
				if err == nil {
					t.Fatal("Expected non-nil error")
				}
				if err.HTTPCode != 503 {
					t.Errorf("Expected HTTP code 503, got %d", err.HTTPCode)
				}
			},
		},
		{
			name: "proto error with agent info",
			pbErr: &pb.DownloadError{
				Code:    int32(ErrCodeGRPCConnectionFailed),
				Method:  "DownloadPart",
				AgentId: 123,
			},
			validate: func(t *testing.T, err *DownloadError) {
				if err == nil {
					t.Fatal("Expected non-nil error")
				}
				if err.Method != "DownloadPart" {
					t.Errorf("Expected method 'DownloadPart', got '%s'", err.Method)
				}
				if err.AgentID != 123 {
					t.Errorf("Expected agent ID 123, got %d", err.AgentID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FromProto(tt.pbErr)
			tt.validate(t, err)
		})
	}
}

func TestRoundTripConversion(t *testing.T) {
	originalErr := &DownloadError{
		Code:     ErrCodeHTTPError,
		Message:  "test error",
		Cause:    fmt.Errorf("underlying cause"),
		HTTPCode: 500,
		Method:   "TestMethod",
		AgentID:  999,
	}

	// Convert to proto and back
	pbErr := originalErr.ToProto()
	convertedErr := FromProto(pbErr)

	if convertedErr.Code != originalErr.Code {
		t.Errorf("Code mismatch: expected %d, got %d", originalErr.Code, convertedErr.Code)
	}
	if convertedErr.Message != originalErr.Message {
		t.Errorf("Message mismatch: expected %s, got %s", originalErr.Message, convertedErr.Message)
	}
	if convertedErr.Cause.Error() != originalErr.Cause.Error() {
		t.Errorf("Cause mismatch: expected %s, got %s", originalErr.Cause.Error(), convertedErr.Cause.Error())
	}
	if convertedErr.HTTPCode != originalErr.HTTPCode {
		t.Errorf("HTTPCode mismatch: expected %d, got %d", originalErr.HTTPCode, convertedErr.HTTPCode)
	}
	if convertedErr.Method != originalErr.Method {
		t.Errorf("Method mismatch: expected %s, got %s", originalErr.Method, convertedErr.Method)
	}
	if convertedErr.AgentID != originalErr.AgentID {
		t.Errorf("AgentID mismatch: expected %d, got %d", originalErr.AgentID, convertedErr.AgentID)
	}
}
