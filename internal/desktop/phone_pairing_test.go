package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrCreatePairingTokenPersistsPrivateCredential(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "mobile-token")
	first, err := loadOrCreatePairingToken(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := loadOrCreatePairingToken(path)
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || first != second {
		t.Fatalf("token did not persist: first=%q second=%q", first, second)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("token mode = %o", info.Mode().Perm())
	}
}
