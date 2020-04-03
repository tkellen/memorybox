package memorybox

import (
	"testing"
)

func TestNotOnDisk(t *testing.T) {
	type testCase struct {
		input    string
		expected bool
	}
	tests := map[string]testCase{
		"standard in input": testCase{"-", true},
		"https input":       testCase{"https://domain.com/image.jpg", true},
		"http input":        testCase{"https://domain.com/image.jpg", true},
	}
	for name, test := range tests {
		name := name
		t.Run(name, func(t *testing.T) {
			actual := notOnDisk(test.input)
			if actual != test.expected {
				t.Fatalf("expected %t, got %t", test.expected, actual)
			}
		})
	}
}
