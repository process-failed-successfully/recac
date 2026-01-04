package validation

import (
	"testing"
)

type TestStruct struct {
	Name     string `json:"name" validate:"required,min=3,max=50"`
	Email    string `json:"email" validate:"required,email"`
	Age      int    `json:"age"`
	Website  string `json:"website" validate:"url"`
	Password string `json:"password" validate:"required,min=8"`
}

func TestValidator(t *testing.T) {
	t.Run("Test Required Validation", func(t *testing.T) {
		v := NewValidator()
		v.Required("", "name")
		v.Required(nil, "email")
		v.Required([]string{}, "tags")

		if v.Validate() {
			t.Error("Expected validation to fail")
		}

		if len(v.Errors()) != 3 {
			t.Errorf("Expected 3 errors, got %d", len(v.Errors()))
		}
	})

	t.Run("Test String Length Validation", func(t *testing.T) {
		v := NewValidator()
		v.MinLength("ab", 3, "name")
		v.MaxLength("abcdefghijklmnopqrstuvwxyz", 5, "name")

		if v.Validate() {
			t.Error("Expected validation to fail")
		}

		if len(v.Errors()) != 2 {
			t.Errorf("Expected 2 errors, got %d", len(v.Errors()))
		}
	})

	t.Run("Test Email Validation", func(t *testing.T) {
		v := NewValidator()
		v.Email("invalid-email", "email")
		v.Email("another@invalid", "email2")

		if v.Validate() {
			t.Error("Expected validation to fail")
		}

		if len(v.Errors()) != 2 {
			t.Errorf("Expected 2 errors, got %d", len(v.Errors()))
		}
	})

	t.Run("Test URL Validation", func(t *testing.T) {
		v := NewValidator()
		v.URL("not-a-url", "website")
		v.URL("http://valid.com", "website2")

		if v.Validate() {
			t.Error("Expected validation to fail for first URL")
		}

		if len(v.Errors()) != 1 {
			t.Errorf("Expected 1 error, got %d", len(v.Errors()))
		}
	})

	t.Run("Test Struct Validation", func(t *testing.T) {
		testData := TestStruct{
			Name:    "Jo",
			Email:   "invalid",
			Website: "not-a-url",
		}

		v := NewValidator()
		err := v.ValidateStruct(testData)

		if err == nil {
			t.Error("Expected validation error")
		}

		if len(v.Errors()) != 3 {
			t.Errorf("Expected 3 errors, got %d", len(v.Errors()))
		}
	})

	t.Run("Test Valid Struct", func(t *testing.T) {
		testData := TestStruct{
			Name:    "John Doe",
			Email:   "john@example.com",
			Website: "https://example.com",
			Password: "password123",
		}

		v := NewValidator()
		err := v.ValidateStruct(testData)

		if err != nil {
			t.Errorf("Expected no validation error, got: %v", err)
		}

		if !v.Validate() {
			t.Error("Expected validation to pass")
		}
	})
}
