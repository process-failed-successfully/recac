package errors

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"
)

// JiraError represents an error from the Jira API
type JiraError struct {
	StatusCode int
	Message    string
	RetryAfter time.Duration
}

// Error implements the error interface
func (e *JiraError) Error() string {
	return fmt.Sprintf("Jira API error (status %d): %s", e.StatusCode, e.Message)
}

// HandleJiraAPIError handles errors from Jira API calls
func HandleJiraAPIError(err error, maxRetries int, retryDelay time.Duration) error {
	var jiraErr *JiraError
	var netErr interface{ Timeout() bool }

	if errors.As(err, &jiraErr) {
		return handleJiraError(jiraErr, maxRetries, retryDelay)
	} else if errors.As(err, &netErr) && netErr.Timeout() {
		return handleNetworkError(err, maxRetries, retryDelay)
	} else if err != nil {
		return handleGenericError(err, maxRetries, retryDelay)
	}

	return nil
}

func handleJiraError(err *JiraError, maxRetries int, retryDelay time.Duration) error {
	log.Printf("Jira API error occurred: %v", err)

	// For rate limiting (429), use the Retry-After header if available
	if err.StatusCode == http.StatusTooManyRequests {
		if err.RetryAfter > 0 {
			log.Printf("Rate limited. Retrying after %v...", err.RetryAfter)
			time.Sleep(err.RetryAfter)
			return nil // Return nil to indicate retry should happen
		}
		log.Printf("Rate limited. Retrying after %v...", retryDelay)
		time.Sleep(retryDelay)
		return nil
	}

	// For server errors (5xx), implement retry logic
	if err.StatusCode >= 500 && err.StatusCode < 600 {
		for i := 0; i < maxRetries; i++ {
			log.Printf("Server error. Retry %d/%d after %v...", i+1, maxRetries, retryDelay)
			time.Sleep(retryDelay)
			// In actual implementation, the retry would happen in the calling function
		}
		return fmt.Errorf("max retries reached for Jira API error: %w", err)
	}

	// For client errors (4xx), don't retry
	return fmt.Errorf("Jira API client error (no retry): %w", err)
}

func handleNetworkError(err error, maxRetries int, retryDelay time.Duration) error {
	log.Printf("Network error occurred: %v", err)

	for i := 0; i < maxRetries; i++ {
		log.Printf("Network error. Retry %d/%d after %v...", i+1, maxRetries, retryDelay)
		time.Sleep(retryDelay)
		// In actual implementation, the retry would happen in the calling function
	}

	return fmt.Errorf("max retries reached for network error: %w", err)
}

func handleGenericError(err error, maxRetries int, retryDelay time.Duration) error {
	log.Printf("Generic error occurred: %v", err)

	for i := 0; i < maxRetries; i++ {
		log.Printf("Generic error. Retry %d/%d after %v...", i+1, maxRetries, retryDelay)
		time.Sleep(retryDelay)
		// In actual implementation, the retry would happen in the calling function
	}

	return fmt.Errorf("max retries reached for generic error: %w", err)
}

// NewJiraError creates a new JiraError
func NewJiraError(statusCode int, message string, retryAfter time.Duration) *JiraError {
	return &JiraError{
		StatusCode: statusCode,
		Message:    message,
		RetryAfter: retryAfter,
	}
}
