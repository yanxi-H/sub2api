package runtime_test

import (
	"testing"

	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/ent/usagelog"
)

func TestUsageLogRuntimeInitialization(t *testing.T) {
	if got := usagelog.DefaultRequestBodyBytes; got != 0 {
		t.Fatalf("unexpected request body bytes default: got %d, want 0", got)
	}
}
