package config

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestNewLoggerWritesJSONWithUppercaseLevel(t *testing.T) {
	var buf bytes.Buffer

	newLogger(&buf, "error").Error("boom", "key", "value")

	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("decode log: %v", err)
	}
	if out["level"] != "ERROR" {
		t.Fatalf("level = %v, want %q", out["level"], "ERROR")
	}
	if out["msg"] != "boom" {
		t.Fatalf("msg = %v, want %q", out["msg"], "boom")
	}
	if out["key"] != "value" {
		t.Fatalf("key = %v, want %q", out["key"], "value")
	}
}

func TestNewLoggerRespectsLevel(t *testing.T) {
	var buf bytes.Buffer

	newLogger(&buf, "error").Info("ignored")

	if buf.Len() != 0 {
		t.Fatalf("log = %q, want empty", buf.String())
	}
}
