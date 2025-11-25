package main

import (
	"testing"
)

func TestDiscoverTests(t *testing.T) {
	tests, err := DiscoverTests("testdata")
	if err != nil {
		t.Fatalf("DiscoverTests failed: %v", err)
	}

	if len(tests) == 0 {
		t.Fatal("Expected to find tests, got none")
	}

	expectedTests := map[string]bool{
		"TestQuickPass":   false,
		"TestSlowPass":    false,
		"TestFail":        false,
		"TestAnotherPass": false,
		"TestWithOutput":  false,
	}

	for _, test := range tests {
		t.Logf("Found test: %s (package: %s)", test.Name, test.Package)
		if _, ok := expectedTests[test.Name]; ok {
			expectedTests[test.Name] = true
		}
	}

	for name, found := range expectedTests {
		if !found {
			t.Errorf("Expected test %s was not found", name)
		}
	}
}
