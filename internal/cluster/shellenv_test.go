package cluster

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestParseShellEnv(t *testing.T) {
	sentinel := "_NAKUBE_ENV_abc_"
	wrap := func(body string) string {
		return "motd\n" + sentinel + "\n" + body + "\n" + sentinel + "\n"
	}

	t.Run("extracts pairs between sentinels", func(t *testing.T) {
		env, err := parseShellEnv(wrap("FOO=bar\nBAZ=qux\n"))
		if err != nil {
			t.Fatal(err)
		}
		if env["FOO"] != "bar" || env["BAZ"] != "qux" {
			t.Fatalf("%v", env)
		}
	})
	t.Run("drops garbage and continuations", func(t *testing.T) {
		env, err := parseShellEnv(wrap("OK=1\nnot a pair\n  indented\nALSO=2\n"))
		if err != nil {
			t.Fatal(err)
		}
		if len(env) != 2 || env["OK"] != "1" || env["ALSO"] != "2" {
			t.Fatalf("%v", env)
		}
	})
	t.Run("later duplicate wins", func(t *testing.T) {
		env, err := parseShellEnv(wrap("FOO=1\nFOO=2\n"))
		if err != nil {
			t.Fatal(err)
		}
		if env["FOO"] != "2" {
			t.Fatalf("FOO = %q", env["FOO"])
		}
	})
	t.Run("missing sentinel errors", func(t *testing.T) {
		if _, err := parseShellEnv("FOO=1\n"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("single sentinel errors", func(t *testing.T) {
		if _, err := parseShellEnv(sentinel + "\nFOO=1\n"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("empty section errors", func(t *testing.T) {
		if _, err := parseShellEnv(sentinel + "\n\n" + sentinel + "\n"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("value containing equals", func(t *testing.T) {
		env, err := parseShellEnv(wrap("FOO=a=b=c\n"))
		if err != nil {
			t.Fatal(err)
		}
		if env["FOO"] != "a=b=c" {
			t.Fatalf("FOO = %q", env["FOO"])
		}
	})
}

func TestMergePathLists(t *testing.T) {
	got := mergePathLists("/opt/homebrew/bin:/usr/bin:/bin", "/usr/local/bin:/usr/bin:/sbin")
	want := "/opt/homebrew/bin:/usr/bin:/bin:/usr/local/bin:/sbin"
	if got != want {
		t.Errorf("mergePathLists = %q, want %q", got, want)
	}
}

func TestShellEnvArgs(t *testing.T) {
	if got := shellEnvArgs("/bin/zsh", "x"); len(got) < 2 || got[0] != "-i" || got[1] != "-l" {
		t.Errorf("zsh args = %v", got)
	}
	if got := shellEnvArgs("/usr/bin/fish", "x"); len(got) == 0 || got[0] != "-i" {
		t.Errorf("fish args = %v", got)
	}
	if got := shellEnvArgs("/bin/dash", "x"); len(got) == 0 || got[0] != "-l" {
		t.Errorf("dash should not get -i: %v", got)
	}
}

func TestMergeShellEnvAllowlist(t *testing.T) {
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("LD_PRELOAD", "")
	t.Setenv("DYLD_INSERT_LIBRARIES", "")
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("RANDOM_SECRET", "")

	mergeShellEnv(map[string]string{
		"AWS_PROFILE":           "dev",
		"LD_PRELOAD":            "/evil.so",
		"DYLD_INSERT_LIBRARIES": "/evil.dylib",
		"PATH":                  "/opt/homebrew/bin",
		"RANDOM_SECRET":         "nope",
	})

	if os.Getenv("AWS_PROFILE") != "dev" {
		t.Fatalf("AWS_PROFILE = %q", os.Getenv("AWS_PROFILE"))
	}
	if got := os.Getenv("LD_PRELOAD"); got != "" {
		t.Fatalf("LD_PRELOAD imported: %q", got)
	}
	if got := os.Getenv("DYLD_INSERT_LIBRARIES"); got != "" {
		t.Fatalf("DYLD_* imported: %q", got)
	}
	if got := os.Getenv("RANDOM_SECRET"); got != "" {
		t.Fatalf("non-allowlist imported: %q", got)
	}
	if !strings.Contains(os.Getenv("PATH"), "/opt/homebrew/bin") {
		t.Fatalf("PATH = %q", os.Getenv("PATH"))
	}
}

func TestImportShellEnvFixture(t *testing.T) {
	if runtime.GOOS == "windows" {
		if err := ImportShellEnv(); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Setenv("TERM", "")
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("LD_PRELOAD", "")

	const secret = "SUPER_SECRET_TOKEN_xyz"
	dump := "motd " + secret + "\n_NAKUBE_ENV_abc_\nAWS_PROFILE=fixture\nLD_PRELOAD=/tmp/evil.so\nPATH=/bin\n_NAKUBE_ENV_abc_\n"
	prev := runLoginShell
	runLoginShell = func(context.Context, string, []string) ([]byte, error) {
		return []byte(dump), nil
	}
	t.Cleanup(func() { runLoginShell = prev })

	if err := ImportShellEnv(); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("AWS_PROFILE") != "fixture" {
		t.Fatalf("AWS_PROFILE = %q", os.Getenv("AWS_PROFILE"))
	}
	if got := os.Getenv("LD_PRELOAD"); got != "" {
		t.Fatalf("LD_PRELOAD = %q", got)
	}
}

func TestImportShellEnvErrorOmitsDump(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows does not spawn a login shell")
	}
	t.Setenv("TERM", "")
	t.Setenv("TERM_PROGRAM", "")
	const secret = "SUPER_SECRET_TOKEN_xyz"
	prev := runLoginShell
	runLoginShell = func(context.Context, string, []string) ([]byte, error) {
		return []byte("garbage " + secret), nil
	}
	t.Cleanup(func() { runLoginShell = prev })

	err := ImportShellEnv()
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error attached the dump: %v", err)
	}
}

func TestImportShellEnvSkipsWhenTERMSet(t *testing.T) {
	t.Setenv("TERM", "xterm")
	called := false
	prev := runLoginShell
	runLoginShell = func(context.Context, string, []string) ([]byte, error) {
		called = true
		return nil, os.ErrInvalid
	}
	t.Cleanup(func() { runLoginShell = prev })

	if err := ImportShellEnv(); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("spawned a shell despite TERM")
	}
}
