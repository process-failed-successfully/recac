package runner

import (
	"fmt"
)

// checkNoOpBreaker checks if the agent is looping without action.
func (s *Session) checkNoOpBreaker(executionOutput string) error {
	if executionOutput == "" {
		s.NoOpCount++
		if s.NoOpCount >= 3 {
			return fmt.Errorf("CIRCUIT BREAKER TRIPPED: %w (Agent has produced 3 consecutive responses with no commands)", ErrNoOp)
		}
	} else {
		s.NoOpCount = 0 // Reset on valid action
	}
	return nil
}

// checkStalledBreaker checks if the agent is making progress on features.
func (s *Session) checkFeatures() int {
	features := s.loadFeatures()
	passed := 0
	for _, f := range features {
		if f.Passes || f.Status == "done" || f.Status == "implemented" {
			passed++
		}
	}
	return passed
}

func (s *Session) checkStalledBreaker(role string, passingCount int) error {
	if role == "manager_review" || role == "Manager" || role == "initializer" {
		s.StalledCount = 0
		s.LastFeatureCount = passingCount
		return nil
	}

	if passingCount == s.LastFeatureCount {
		s.StalledCount++
	} else {
		s.StalledCount = 0
		s.LastFeatureCount = passingCount
	}

	// Trigger Manager if stalled
	if s.StalledCount >= s.ManagerFrequency && s.StalledCount%s.ManagerFrequency == 0 {
		fmt.Printf("Warning: Progress stalled for %d iterations. Summoning Manager.\n", s.StalledCount)
		s.createSignal("TRIGGER_MANAGER")
		s.createSignal("STALLED_WARNING") // Flag for prompt construction
		return nil                        // Give Manager a chance!
	}

	// Hard stop if stalled too long (3x frequency)
	if s.StalledCount >= s.ManagerFrequency*3 {
		return fmt.Errorf("CIRCUIT BREAKER TRIPPED: %w (Stalled for %d iterations)", ErrStalled, s.StalledCount)
	}

	return nil
}
