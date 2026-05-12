package s5

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveGMGNCLIPathFindsAncestorNodeModules(t *testing.T) {
	name := "gmgn-cli"
	if runtime.GOOS == "windows" {
		name = "gmgn-cli.cmd"
	}

	root := t.TempDir()
	cliDir := filepath.Join(root, "node_modules", ".bin")
	if err := os.MkdirAll(cliDir, 0o755); err != nil {
		t.Fatalf("mkdir cli dir: %v", err)
	}
	cliPath := filepath.Join(cliDir, name)
	if err := os.WriteFile(cliPath, []byte("echo"), 0o644); err != nil {
		t.Fatalf("write fake cli: %v", err)
	}

	childDir := filepath.Join(root, "go-radar", "cmd", "radar")
	if err := os.MkdirAll(childDir, 0o755); err != nil {
		t.Fatalf("mkdir child dir: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	if err := os.Chdir(childDir); err != nil {
		t.Fatalf("chdir child dir: %v", err)
	}

	resolved := resolveGMGNCLIPath()
	if resolved != cliPath {
		t.Fatalf("expected %q, got %q", cliPath, resolved)
	}
}
