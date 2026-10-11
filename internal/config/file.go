package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// File is the content of the config file (JSON).
type File struct {
	Upload      UploadDefaults `json:"upload,omitzero"`
	Credentials Credentials    `json:"credentials,omitzero"`
}

// FilePath returns the config file path:
// GY_CONFIG, then $XDG_CONFIG_HOME/gy/config.json, then ~/.config/gy/config.json.
func FilePath(getenv func(string) string) (string, error) {
	if p := getenv(EnvConfigFile); p != "" {
		return p, nil
	}
	if dir := getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "gy", "config.json"), nil
	}
	home := getenv("HOME")
	if home == "" {
		var err error
		if home, err = os.UserHomeDir(); err != nil {
			return "", fmt.Errorf("resolve config path: %w", err)
		}
	}
	return filepath.Join(home, ".config", "gy", "config.json"), nil
}

// ReadFile reads the config file. It returns the zero value if the file does not exist.
func ReadFile(path string) (File, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return File{}, nil
	}
	if err != nil {
		return File{}, fmt.Errorf("read config file: %w", err)
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return File{}, fmt.Errorf("parse config file %s: %w", path, err)
	}
	return f, nil
}

// WriteFile writes the config file, creating the directory with 0700 and the file with 0600.
// It writes to a temp file and renames it, so a failed write never leaves a broken file.
func WriteFile(path string, f File) error {
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".config-*.json")
	if err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	defer os.Remove(tmp.Name())

	// os.CreateTemp creates the file with 0600.
	_, err = tmp.Write(append(b, '\n'))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
}

// UpdateFile reads the config file, applies fn and writes it back.
func UpdateFile(path string, fn func(*File)) error {
	f, err := ReadFile(path)
	if err != nil {
		return err
	}
	fn(&f)
	return WriteFile(path, f)
}

// UploadDefaults are the defaults of 'gy upload' flags.
type UploadDefaults struct {
	// AccessPolicy is gyazo.AccessPolicyAnyone or gyazo.AccessPolicyOnlyMe.
	AccessPolicy string `json:"accessPolicy,omitempty"`
	// App is the application name recorded with uploads.
	App string `json:"app,omitempty"`
}
