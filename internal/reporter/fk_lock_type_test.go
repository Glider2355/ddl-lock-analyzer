package reporter

import (
	"testing"

	"github.com/Glider2355/ddl-lock-analyzer/internal/meta"
)

func TestFKLockTypeString(t *testing.T) {
	tests := []struct {
		name  string
		level meta.LockLevel
		want  string
	}{
		{"EXCLUSIVE → EXCLUSIVE", meta.LockExclusive, "EXCLUSIVE"},
		{"SHARED → SHARED_READ", meta.LockShared, "SHARED_READ"},
		{"NONE → SHARED_READ", meta.LockNone, "SHARED_READ"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FKLockTypeString(tt.level); got != tt.want {
				t.Errorf("FKLockTypeString(%s) = %q, want %q", tt.level, got, tt.want)
			}
		})
	}
}
