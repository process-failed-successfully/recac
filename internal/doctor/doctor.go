package doctor

import (
	"context"
	"time"
)

// CheckResult represents the result of a single health check.
type CheckResult struct {
	Name      string
	Passed    bool
	Skipped   bool
	Message   string
	Error     error
	Timestamp time.Time
}

// Check is the interface for a health check.
type Check interface {
	Name() string
	Run(ctx context.Context) CheckResult
}

// Doctor manages a suite of checks.
type Doctor struct {
	checks []Check
}

// NewDoctor creates a new Doctor instance.
func NewDoctor() *Doctor {
	return &Doctor{
		checks: []Check{},
	}
}

// AddCheck adds a check to the suite.
func (d *Doctor) AddCheck(c Check) {
	d.checks = append(d.checks, c)
}

// RunChecks executes all registered checks concurrently.
func (d *Doctor) RunChecks(ctx context.Context) []CheckResult {
	results := make([]CheckResult, len(d.checks))
	errChan := make(chan struct {
		idx int
		res CheckResult
	}, len(d.checks))

	for i, check := range d.checks {
		go func(idx int, c Check) {
			res := c.Run(ctx)
			res.Timestamp = time.Now()
			errChan <- struct {
				idx int
				res CheckResult
			}{idx, res}
		}(i, check)
	}

	for i := 0; i < len(d.checks); i++ {
		completed := <-errChan
		results[completed.idx] = completed.res
	}

	return results
}

// Helper to format error messages
func formatError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// BaseCheck provides common functionality for checks.
type BaseCheck struct {
	CheckName string
}

func (b *BaseCheck) Name() string {
	return b.CheckName
}

func (b *BaseCheck) Success(msg string) CheckResult {
	return CheckResult{
		Name:    b.CheckName,
		Passed:  true,
		Message: msg,
	}
}

func (b *BaseCheck) Fail(err error) CheckResult {
	return CheckResult{
		Name:    b.CheckName,
		Passed:  false,
		Error:   err,
		Message: formatError(err),
	}
}

func (b *BaseCheck) Skip(msg string) CheckResult {
	return CheckResult{
		Name:    b.CheckName,
		Skipped: true,
		Passed:  true, // Skipped is technically not a failure
		Message: msg,
	}
}
