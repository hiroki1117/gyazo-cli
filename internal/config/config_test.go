package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func envFunc(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoad_Precedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := WriteFile(path, File{Upload: UploadDefaults{App: "gy"}, Credentials: Credentials{AccessToken: "filetoken"}}); err != nil {
		t.Fatal(err)
	}

	t.Run("file only", func(t *testing.T) {
		cfg, err := Load(envFunc(map[string]string{EnvConfigFile: path}))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Upload.App != "gy" || cfg.CredentialSource != SourceFile || cfg.Credentials.AccessToken != "filetoken" {
			t.Errorf("unexpected config: %+v", cfg)
		}
	})

	t.Run("env overrides file", func(t *testing.T) {
		cfg, err := Load(envFunc(map[string]string{EnvConfigFile: path, EnvAccessToken: "envtoken", EnvMaxRetries: "0"}))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.CredentialSource != SourceEnv || cfg.Credentials.AccessToken != "envtoken" || cfg.MaxRetries != 0 {
			t.Errorf("unexpected config: %+v", cfg)
		}
	})

	t.Run("invalid env", func(t *testing.T) {
		if _, err := Load(envFunc(map[string]string{EnvConfigFile: path, EnvTimeout: "soon"})); err == nil {
			t.Error("expected error for invalid timeout")
		}
	})
}

func TestLoad_MissingFile(t *testing.T) {
	cfg, err := Load(envFunc(map[string]string{EnvConfigFile: filepath.Join(t.TempDir(), "none.json")}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CredentialSource != SourceNone || !cfg.Credentials.IsZero() {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestWriteFile_Permission(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gy", "config.json")
	if err := UpdateFile(path, func(f *File) { f.Credentials = Credentials{AccessToken: "tok"} }); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows has no permission bits.
	if perm := info.Mode().Perm(); runtime.GOOS != "windows" && perm != 0o600 {
		t.Errorf("perm = %o, want 600", perm)
	}
	f, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if f.Credentials.AccessToken != "tok" {
		t.Errorf("credentials = %+v", f.Credentials)
	}
}

func TestFilePath(t *testing.T) {
	tests := []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{EnvConfigFile: "/x/c.json", "HOME": "/h"}, "/x/c.json"},
		{map[string]string{"XDG_CONFIG_HOME": "/xdg", "HOME": "/h"}, filepath.Join("/xdg", "gy", "config.json")},
		{map[string]string{"HOME": "/h"}, filepath.Join("/h", ".config", "gy", "config.json")},
	}
	for _, tt := range tests {
		got, err := FilePath(envFunc(tt.env))
		if err != nil {
			t.Fatal(err)
		}
		if got != tt.want {
			t.Errorf("FilePath(%v) = %q, want %q", tt.env, got, tt.want)
		}
	}
}
