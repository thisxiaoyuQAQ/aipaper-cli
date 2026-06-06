package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type WriteMode int

const (
	CreateOnly WriteMode = iota
	Overwrite
)

type WriteResult struct {
	Path          string
	SHA256        string
	AlreadyExists bool
}

func WriteJSON(path string, value any, mode WriteMode) (WriteResult, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return WriteResult{}, err
	}
	data = append(data, '\n')
	if !json.Valid(data) {
		return WriteResult{}, fmt.Errorf("marshaled JSON for %s is invalid", path)
	}
	return WriteFile(path, data, mode)
}

func ReadJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(value); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	var extra any
	if err := dec.Decode(&extra); err == nil {
		return fmt.Errorf("parse %s: trailing JSON value", path)
	} else if !errors.Is(err, io.EOF) {
		return fmt.Errorf("parse %s: trailing JSON data: %w", path, err)
	}
	return nil
}

func WriteFile(path string, data []byte, mode WriteMode) (WriteResult, error) {
	if path == "" {
		return WriteResult{}, errors.New("path is required")
	}
	sum := SHA256(data)
	if existing, err := os.ReadFile(path); err == nil {
		if mode == CreateOnly {
			if SHA256(existing) == sum {
				return WriteResult{Path: path, SHA256: sum, AlreadyExists: true}, nil
			}
			return WriteResult{}, fmt.Errorf("artifact conflict at %s", path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return WriteResult{}, err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return WriteResult{}, err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return WriteResult{}, err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return WriteResult{}, err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return WriteResult{}, err
	}
	if err := tmp.Close(); err != nil {
		return WriteResult{}, err
	}
	if err := replaceFile(tmpPath, path); err != nil {
		return WriteResult{}, err
	}
	cleanup = false
	_ = syncDir(dir)
	return WriteResult{Path: path, SHA256: sum}, nil
}

func EnsureLayout(s Store) error {
	if err := os.MkdirAll(s.Root(), 0o755); err != nil {
		return err
	}
	for _, dir := range RequiredDirs() {
		if err := os.MkdirAll(s.Path(dir), 0o755); err != nil {
			return err
		}
	}
	return nil
}

func syncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
