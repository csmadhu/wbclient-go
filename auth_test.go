package wbclientgo

import (
	"context"
	"fmt"
	"os"
	"testing"
)

// TestAuthWithPlainText tests plain text authentication
// Subtests: correct password, wrong password, wrong username
func TestAuthWithPlainText(t *testing.T) {
	// Get test credentials from environment variables
	username := os.Getenv("TEST_USERNAME")
	domain := os.Getenv("TEST_DOMAIN")
	password := os.Getenv("TEST_PASSWORD")

	// Skip test if environment variables are not set
	if username == "" || domain == "" || password == "" {
		t.Skip("Skipping TestAuthWithPlainText: TEST_USERNAME, TEST_DOMAIN, and TEST_PASSWORD environment variables must be set")
	}

	ctx := context.Background()

	t.Run("CorrectPassword", func(t *testing.T) {
		req := UserValidateReq{
			Username:        username,
			Password:        password,
			Domain:          domain,
			IsPlainTextAuth: true,
		}

		result := AuthenticateWithPlainText(ctx, req)

		if !result.Success {
			t.Errorf("Expected authentication to succeed with correct password, but got: ErrorCode=%d, ErrorMessage=%s",
				result.ErrorCode, result.ErrorMessage)
		}

		if result.ErrorCode != 0 {
			t.Errorf("Expected ErrorCode=0, got ErrorCode=%d", result.ErrorCode)
		}
	})

	t.Run("WrongPassword", func(t *testing.T) {
		req := UserValidateReq{
			Username:        username,
			Password:        fmt.Sprintf("wrong%s", password),
			Domain:          domain,
			IsPlainTextAuth: true,
		}

		result := AuthenticateWithPlainText(ctx, req)

		if result.Success {
			t.Error("Expected authentication to fail with wrong password, but it succeeded")
		}

		if result.ErrorCode == 0 {
			t.Error("Expected non-zero ErrorCode for wrong password")
		}

		if result.ErrorMessage == "" {
			t.Error("Expected error message for wrong password")
		}
	})

	t.Run("WrongUsername", func(t *testing.T) {
		req := UserValidateReq{
			Username:        fmt.Sprintf("wrong%s", username),
			Password:        password,
			Domain:          domain,
			IsPlainTextAuth: true,
		}

		result := AuthenticateWithPlainText(ctx, req)

		if result.Success {
			t.Error("Expected authentication to fail with wrong username, but it succeeded")
		}

		if result.ErrorCode == 0 {
			t.Error("Expected non-zero ErrorCode for wrong username")
		}

		if result.ErrorMessage == "" {
			t.Error("Expected error message for wrong username")
		}
	})
}

// TestAuthWithChallenge tests MSCHAPv2 authentication
// Subtests: correct password, wrong password, wrong username
func TestAuthWithChallenge(t *testing.T) {
	// Get test credentials from environment variables
	username := os.Getenv("TEST_USERNAME")
	domain := os.Getenv("TEST_DOMAIN")
	password := os.Getenv("TEST_PASSWORD")

	// Skip test if environment variables are not set
	if username == "" || domain == "" || password == "" {
		t.Skip("Skipping TestAuthWithChallenge: TEST_USERNAME, TEST_DOMAIN, and TEST_PASSWORD environment variables must be set")
	}

	ctx := context.Background()

	t.Run("CorrectPassword", func(t *testing.T) {
		req := UserValidateReq{
			Username:        username,
			Password:        password,
			Domain:          domain,
			IsPlainTextAuth: false,
		}

		result := AuthenticateWithChallenge(ctx, req)

		if !result.Success {
			t.Errorf("Expected authentication to succeed with correct password, but got: ErrorCode=%d, ErrorMessage=%s",
				result.ErrorCode, result.ErrorMessage)
		}

		if result.ErrorCode != 0 {
			t.Errorf("Expected ErrorCode=0, got ErrorCode=%d", result.ErrorCode)
		}
	})

	t.Run("WrongPassword", func(t *testing.T) {
		req := UserValidateReq{
			Username:        username,
			Password:        "WrongPassword123!",
			Domain:          domain,
			IsPlainTextAuth: false,
		}

		result := AuthenticateWithChallenge(ctx, req)

		if result.Success {
			t.Error("Expected authentication to fail with wrong password, but it succeeded")
		}

		if result.ErrorCode == 0 {
			t.Error("Expected non-zero ErrorCode for wrong password")
		}

		if result.ErrorMessage == "" {
			t.Error("Expected error message for wrong password")
		}
	})

	t.Run("WrongUsername", func(t *testing.T) {
		req := UserValidateReq{
			Username:        "nonexistentuser",
			Password:        password,
			Domain:          domain,
			IsPlainTextAuth: false,
		}

		result := AuthenticateWithChallenge(ctx, req)

		if result.Success {
			t.Error("Expected authentication to fail with wrong username, but it succeeded")
		}

		if result.ErrorCode == 0 {
			t.Error("Expected non-zero ErrorCode for wrong username")
		}

		if result.ErrorMessage == "" {
			t.Error("Expected error message for wrong username")
		}
	})
}
