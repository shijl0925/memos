package api

import "testing"

func TestMaxContentLengthIsOneMegabyte(t *testing.T) {
	if MaxContentLength != 1<<20 {
		t.Fatalf("MaxContentLength = %d, want %d", MaxContentLength, 1<<20)
	}
}
