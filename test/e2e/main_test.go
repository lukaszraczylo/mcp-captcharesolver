//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"testing"
)

func TestMain(m *testing.M) {
	build := exec.Command("go", "build", "-o", "../../bin/captcha-solver-mcp", "../../cmd/captcha-solver-mcp")
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		os.Exit(1)
	}
	os.Exit(m.Run())
}
