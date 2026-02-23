package scenarios

import (
	"testing"
)

func TestDistributedLogScenario_Generate(t *testing.T) {
	s := &DistributedLogScenario{}

	if s.Name() != "distributed-log" {
		t.Errorf("Expected name distributed-log")
	}

	if s.Description() == "" {
		t.Error("Expected description")
	}

	spec := s.AppSpec("http://repo")
	if spec == "" {
		t.Error("Expected spec")
	}

	tickets := s.Generate("uid", "http://repo")
	if len(tickets) != 1 {
		t.Errorf("Expected 1 ticket, got %d", len(tickets))
	}

	if tickets[0].ID != "LOG" {
		t.Errorf("Expected ticket LOG")
	}
}

func TestLoadBalancerScenario_Generate(t *testing.T) {
	s := &LoadBalancerScenario{}

	if s.Name() != "load-balancer" {
		t.Errorf("Expected name load-balancer")
	}

	tickets := s.Generate("uid", "http://repo")
	if len(tickets) != 1 {
		t.Errorf("Expected 1 ticket, got %d", len(tickets))
	}
}

func TestPrimePythonScenario_Generate(t *testing.T) {
	s := &PrimePythonScenario{}

	if s.Name() != "prime-python" {
		t.Errorf("Expected name prime-python")
	}

	tickets := s.Generate("uid", "http://repo")
	if len(tickets) != 1 {
		t.Errorf("Expected 1 ticket, got %d", len(tickets))
	}
}

func TestHTTPProxyScenario_Generate(t *testing.T) {
	s := &HTTPProxyScenario{}

	if s.Name() != "http-proxy" {
		t.Errorf("Expected name http-proxy")
	}

	tickets := s.Generate("uid", "http://repo")
	if len(tickets) == 0 {
		t.Errorf("Expected tickets")
	}
}

func TestRedisChallengeScenario_Generate(t *testing.T) {
	s := &RedisChallengeScenario{}

	if s.Name() != "redis-challenge" {
		t.Errorf("Expected name redis-challenge")
	}

	tickets := s.Generate("uid", "http://repo")
	if len(tickets) != 1 {
		t.Errorf("Expected 1 ticket, got %d", len(tickets))
	}
}

func TestSQLParserScenario_Generate(t *testing.T) {
	s := &SQLParserScenario{}

	if s.Name() != "sql-parser" {
		t.Errorf("Expected name sql-parser")
	}

	tickets := s.Generate("uid", "http://repo")
	if len(tickets) != 1 {
		t.Errorf("Expected 1 ticket, got %d", len(tickets))
	}
}
