package testdata

import (
	"testing"
	"time"
)

func TestQuickPass(t *testing.T) {
	t.Log("This test passes quickly")
}

func TestSlowPass(t *testing.T) {
	t.Log("This test takes 2 seconds")
	time.Sleep(2 * time.Second)
	t.Log("Done!")
}

func TestFail(t *testing.T) {
	t.Log("This test will fail")
	t.Error("intentional failure")
}

func TestAnotherPass(t *testing.T) {
	t.Log("Another passing test")
}

func TestWithOutput(t *testing.T) {
	for i := 0; i < 10; i++ {
		t.Logf("Output line %d", i)
		time.Sleep(100 * time.Millisecond)
	}
}
