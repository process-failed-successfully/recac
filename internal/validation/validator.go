package validation

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Validator provides methods for validating various types of input
type Validator struct {
	errors map[string]string
}

// NewValidator creates a new Validator instance
func NewValidator() *Validator {
	return &Validator{
		errors: make(map[string]string),
	}
}

// Validate checks if there are any validation errors
func (v *Validator) Validate() bool {
	return len(v.errors) == 0
}

// Errors returns the validation errors
func (v *Validator) Errors() map[string]string {
	return v.errors
}

// AddError adds a validation error
func (v *Validator) AddError(key, message string) {
	if _, exists := v.errors[key]; !exists {
		v.errors[key] = message
	}
}

// ClearErrors clears all validation errors
func (v *Validator) ClearErrors() {
	v.errors = make(map[string]string)
}

// Required checks if a value is not empty
func (v *Validator) Required(value interface{}, fieldName string) {
	if value == nil {
		v.AddError(fieldName, fmt.Sprintf("%s is required", fieldName))
		return
	}

	val := reflect.ValueOf(value)
	switch val.Kind() {
	case reflect.String:
		if strings.TrimSpace(val.String()) == "" {
			v.AddError(fieldName, fmt.Sprintf("%s is required", fieldName))
		}
	case reflect.Slice, reflect.Map, reflect.Array:
		if val.Len() == 0 {
			v.AddError(fieldName, fmt.Sprintf("%s is required", fieldName))
		}
	case reflect.Ptr:
		if val.IsNil() {
			v.AddError(fieldName, fmt.Sprintf("%s is required", fieldName))
		}
	}
}

// MinLength checks if a string has minimum length
func (v *Validator) MinLength(value string, min int, fieldName string) {
	if len(strings.TrimSpace(value)) < min {
		v.AddError(fieldName, fmt.Sprintf("%s must be at least %d characters", fieldName, min))
	}
}

// MaxLength checks if a string has maximum length
func (v *Validator) MaxLength(value string, max int, fieldName string) {
	if len(value) > max {
		v.AddError(fieldName, fmt.Sprintf("%s must be at most %d characters", fieldName, max))
	}
}

// Email validates an email address format
func (v *Validator) Email(value string, fieldName string) {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(value) {
		v.AddError(fieldName, fmt.Sprintf("%s must be a valid email address", fieldName))
	}
}

// URL validates a URL format
func (v *Validator) URL(value string, fieldName string) {
	urlRegex := regexp.MustCompile(`^(https?|ftp):\/\/[^\s/$.?#].[^\s]*$`)
	if !urlRegex.MatchString(value) {
		v.AddError(fieldName, fmt.Sprintf("%s must be a valid URL", fieldName))
	}
}

// Integer validates if a string can be converted to an integer
func (v *Validator) Integer(value string, fieldName string) {
	if _, err := strconv.Atoi(value); err != nil {
		v.AddError(fieldName, fmt.Sprintf("%s must be a valid integer", fieldName))
	}
}

// Float validates if a string can be converted to a float
func (v *Validator) Float(value string, fieldName string) {
	if _, err := strconv.ParseFloat(value, 64); err != nil {
		v.AddError(fieldName, fmt.Sprintf("%s must be a valid number", fieldName))
	}
}

// Date validates if a string is a valid date in YYYY-MM-DD format
func (v *Validator) Date(value string, fieldName string) {
	_, err := time.Parse("2006-01-02", value)
	if err != nil {
		v.AddError(fieldName, fmt.Sprintf("%s must be a valid date (YYYY-MM-DD)", fieldName))
	}
}

// DateTime validates if a string is a valid datetime in RFC3339 format
func (v *Validator) DateTime(value string, fieldName string) {
	_, err := time.Parse(time.RFC3339, value)
	if err != nil {
		v.AddError(fieldName, fmt.Sprintf("%s must be a valid datetime (RFC3339)", fieldName))
	}
}

// OneOf validates if a value is one of the allowed values
func (v *Validator) OneOf(value string, allowed []string, fieldName string) {
	for _, a := range allowed {
		if value == a {
			return
		}
	}
	v.AddError(fieldName, fmt.Sprintf("%s must be one of: %s", fieldName, strings.Join(allowed, ", ")))
}

// Regex validates if a value matches a regular expression
func (v *Validator) Regex(value string, pattern string, fieldName string) {
	regex := regexp.MustCompile(pattern)
	if !regex.MatchString(value) {
		v.AddError(fieldName, fmt.Sprintf("%s must match pattern: %s", fieldName, pattern))
	}
}

// ValidateStruct validates a struct using tags
func (v *Validator) ValidateStruct(s interface{}) error {
	if s == nil {
		return errors.New("cannot validate nil struct")
	}

	val := reflect.ValueOf(s)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return errors.New("can only validate structs")
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := val.Field(i)

		// Get validation tags
		validateTag := field.Tag.Get("validate")
		if validateTag == "" {
			continue
		}

		fieldName := field.Name
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" {
			fieldName = jsonTag
		}

		// Parse validation rules
		rules := strings.Split(validateTag, ",")
		for _, rule := range rules {
			switch rule {
			case "required":
				v.Required(fieldValue.Interface(), fieldName)
			case "email":
				if strVal, ok := fieldValue.Interface().(string); ok {
					v.Email(strVal, fieldName)
				}
			case "url":
				if strVal, ok := fieldValue.Interface().(string); ok {
					v.URL(strVal, fieldName)
				}
			default:
				if strings.HasPrefix(rule, "min=") {
					min := strings.TrimPrefix(rule, "min=")
					if strVal, ok := fieldValue.Interface().(string); ok {
						v.MinLength(strVal, mustAtoi(min), fieldName)
					}
				} else if strings.HasPrefix(rule, "max=") {
					max := strings.TrimPrefix(rule, "max=")
					if strVal, ok := fieldValue.Interface().(string); ok {
						v.MaxLength(strVal, mustAtoi(max), fieldName)
					}
				} else if strings.HasPrefix(rule, "regex=") {
					pattern := strings.TrimPrefix(rule, "regex=")
					if strVal, ok := fieldValue.Interface().(string); ok {
						v.Regex(strVal, pattern, fieldName)
					}
				}
			}
		}
	}

	if !v.Validate() {
		return errors.New("validation failed")
	}
	return nil
}

func mustAtoi(s string) int {
	val, _ := strconv.Atoi(s)
	return val
}
