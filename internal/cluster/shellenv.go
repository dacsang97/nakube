package cluster

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const shellEnvTimeout = 5 * time.Second

// shellEnvAllowlist is the only set of variables imported from a login shell
// besides PATH. DYLD_*, LD_PRELOAD, and the rest of the dump stay out.
var shellEnvAllowlist = []string{
	"AWS_VAULT_BACKEND",
	"AWS_PROFILE",
	"AWS_CONFIG_FILE",
	"AWS_SHARED_CREDENTIALS_FILE",
	"AWS_CA_BUNDLE",
	"SSL_CERT_FILE",
	"CLOUDSDK_CONFIG",
	"AZURE_CONFIG_DIR",
	"HTTPS_PROXY",
	"HTTP_PROXY",
	"NO_PROXY",
	"https_proxy",
	"http_proxy",
	"no_proxy",
	"KUBECONFIG",
}

var (
	envLineRe      = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	sentinelLineRe = regexp.MustCompile(`(?m)^_NAKUBE_ENV_[0-9a-fA-F]+_$`)
)

// runLoginShell is replaced in tests. Production runs the user's login shell.
var runLoginShell = execLoginShell

// ImportShellEnv merges PATH and the credential allowlist from the user's
// login shell. It returns immediately when TERM or TERM_PROGRAM is set, and
// on Windows. Errors never include the shell dump.
func ImportShellEnv() error {
	if runtime.GOOS == "windows" || os.Getenv("TERM") != "" || os.Getenv("TERM_PROGRAM") != "" {
		return nil
	}
	out, err := runLoginShell(context.Background(), userShell(), nil)
	if err != nil {
		return errors.New("login shell env import failed")
	}
	env, err := parseShellEnv(string(out))
	if err != nil {
		return err
	}
	mergeShellEnv(env)
	return nil
}

func execLoginShell(ctx context.Context, shell string, _ []string) ([]byte, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	sentinel := "_NAKUBE_ENV_" + hex.EncodeToString(buf) + "_"
	script := fmt.Sprintf("echo %s; command env; echo %s", sentinel, sentinel)
	cctx, cancel := context.WithTimeout(ctx, shellEnvTimeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, shell, shellEnvArgs(shell, script)...)
	// An interactive shell may leave children holding stdout open after the
	// kill. WaitDelay stops Output from blocking on them.
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return out, nil
}

func userShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	if runtime.GOOS == "darwin" {
		return "/bin/zsh"
	}
	return "/bin/bash"
}

func shellEnvArgs(shell, script string) []string {
	switch filepath.Base(shell) {
	case "zsh", "bash", "fish":
		return []string{"-i", "-l", "-c", script}
	default:
		return []string{"-l", "-c", script}
	}
}

func parseShellEnv(output string) (map[string]string, error) {
	locs := sentinelLineRe.FindAllStringIndex(output, -1)
	if len(locs) < 2 {
		return nil, errors.New("shell env dump missing sentinel")
	}
	section := output[locs[0][1]:locs[len(locs)-1][0]]

	env := make(map[string]string)
	for line := range strings.SplitSeq(section, "\n") {
		if !envLineRe.MatchString(line) {
			continue
		}
		key, value, _ := strings.Cut(line, "=")
		env[key] = value
	}
	if len(env) == 0 {
		return nil, errors.New("shell env dump empty")
	}
	return env, nil
}

func mergeShellEnv(env map[string]string) {
	if shellPath := env["PATH"]; shellPath != "" {
		_ = os.Setenv("PATH", mergePathLists(shellPath, os.Getenv("PATH")))
	}
	for _, key := range shellEnvAllowlist {
		if v := env[key]; v != "" {
			_ = os.Setenv(key, v)
		}
	}
}

func mergePathLists(shellPath, currentPath string) string {
	seen := make(map[string]bool)
	merged := make([]string, 0)
	for _, list := range []string{shellPath, currentPath} {
		for _, dir := range filepath.SplitList(list) {
			if dir == "" || seen[dir] {
				continue
			}
			seen[dir] = true
			merged = append(merged, dir)
		}
	}
	return strings.Join(merged, string(filepath.ListSeparator))
}
