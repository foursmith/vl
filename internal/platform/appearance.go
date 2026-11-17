package platform

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

func HasSystemAppearance() bool { return runtime.GOOS == "darwin" }

// SystemAppearance reads macOS appearance without requiring Automation access.
// A missing AppleInterfaceStyle key means the default light appearance.
func SystemAppearance(ctx context.Context) (light, known bool) {
	if !HasSystemAppearance() {
		return false, false
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "/usr/bin/defaults", "read", "-g", "AppleInterfaceStyle").CombinedOutput()
	if ctx.Err() != nil {
		return false, false
	}
	if err == nil {
		return !strings.EqualFold(strings.TrimSpace(string(output)), "Dark"), true
	}
	if strings.Contains(string(output), "does not exist") {
		return true, true
	}
	return false, false
}
