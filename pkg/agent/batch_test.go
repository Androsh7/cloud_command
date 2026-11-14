package agent

import "testing"

func TestBatchNamesNumbersFromOne(t *testing.T) {
	names := BatchNames("web", 3)

	if len(names) != 3 || names[0] != "web-1" || names[1] != "web-2" || names[2] != "web-3" {
		t.Fatalf("got %v", names)
	}
}

func TestBatchNamesReturnsEmptyForZero(t *testing.T) {
	names := BatchNames("web", 0)

	if len(names) != 0 {
		t.Fatalf("got %v", names)
	}
}
