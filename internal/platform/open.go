package platform

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"time"
)

func OpenURL(ctx context.Context, target string) error {
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("invalid HTTPS URL")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", target)
	case "windows":
		cmd = exec.CommandContext(ctx, "rundll32.exe", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.CommandContext(ctx, "xdg-open", target)
	}
	return cmd.Run()
}
