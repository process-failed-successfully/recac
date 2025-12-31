package runner

import (
	"testing"
)

func TestTruncateRepetitiveResponse(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedOutput string
		wasTruncated   bool
	}{
		{
			name:           "No repetition",
			input:          "Line 1\nLine 2\nLine 3",
			expectedOutput: "Line 1\nLine 2\nLine 3",
			wasTruncated:   false,
		},
		{
			name:           "Single line loop",
			input:          "A\nA\nA\nA\nA\nA\nA\nA\nA\nA\nA\nA", // 12 A's
			expectedOutput: "A",
			wasTruncated:   true,
		},
		{
			name: "Two line pattern loop",
			input: "Start\n" +
				"L1\nL2\n" +
				"L1\nL2\n" +
				"L1\nL2\n" +
				"L1\nL2\n" +
				"L1\nL2\n" +
				"L1\nL2\n" +
				"End",
			expectedOutput: "Start\nL1\nL2",
			wasTruncated:   true,
		},
		{
			name:           "Legitimate repetition (fewer than threshold)",
			input:          "File.txt\nFile.txt\nFile.txt",
			expectedOutput: "File.txt\nFile.txt\nFile.txt",
			wasTruncated:   false,
		},
		{
			name:           "Empty lines loop",
			input:          "\n\n\n\n\n\n\n\n\n\n\n\n",
			expectedOutput: "\n\n\n\n\n\n\n\n\n\n\n\n", // Should ignore empty lines loops for now (or maybe not?)
			wasTruncated:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, truncated := TruncateRepetitiveResponse(tt.input)
			if truncated != tt.wasTruncated {
				t.Errorf("TruncateRepetitiveResponse() truncated = %v, want %v", truncated, tt.wasTruncated)
			}
			if got != tt.expectedOutput {
				t.Errorf("TruncateRepetitiveResponse() got = %v, want %v", got, tt.expectedOutput)
			}
		})
	}
}
