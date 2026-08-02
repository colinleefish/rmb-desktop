package memory

import (
	"testing"
)

func TestParseDistillResponse(t *testing.T) {
	pm, err := parseDistillResponse(`{"abstract":"short","body":"long body"}`)
	if err != nil {
		t.Fatal(err)
	}
	if pm.Abstract != "short" || pm.Body != "long body" {
		t.Fatalf("got %+v", pm)
	}
}
