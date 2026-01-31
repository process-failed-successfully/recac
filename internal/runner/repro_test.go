package runner

import (
	"regexp"
	"testing"
)

func TestReproRegex(t *testing.T) {
	// Copy of the regex from executor.go
	var bashBlockRegex = regexp.MustCompile("(?s)```bash\\s*(.*?)\\s*```")

	// Copy of the response from internal/agent/mock.go
	response := "Here is the implementation for primes.py:\n\n```bash\ncat << 'EOF' > primes.py\nimport json\n\ndef is_prime(n):\n    if n <= 1: return False\n    for i in range(2, int(n**0.5) + 1):\n        if n % i == 0: return False\n    return True\n\nprimes = [x for x in range(10000) if is_prime(x)]\nwith open('primes.json', 'w') as f:\n    json.dump({\"primes\": primes}, f)\nEOF\n\npython3 primes.py\ngit add primes.py\ngit add -f primes.json\ngit commit -m \"Add primes script and output\"\n```"

	matches := bashBlockRegex.FindAllStringSubmatch(response, -1)
	if len(matches) == 0 {
		t.Fatalf("Regex failed to match response:\n%s", response)
	}

	if len(matches[0]) < 2 {
		t.Fatalf("Regex matched but failed to capture group")
	}

	t.Logf("Matched content: %s", matches[0][1])
}
