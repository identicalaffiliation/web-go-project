package logger_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/identicalaffiliation/web-go-project/auth/pkg/logger"
)

func parseLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("log line is not JSON: %q: %v", line, err)
		}
		out = append(out, rec)
	}
	return out
}

func TestLevels(t *testing.T) {
	tests := []struct {
		level slog.Level
		want  []string
	}{
		{slog.LevelDebug, []string{"DEBUG", "INFO", "WARN", "ERROR"}},
		{slog.LevelInfo, []string{"INFO", "WARN", "ERROR"}},
		{slog.LevelWarn, []string{"WARN", "ERROR"}},
		{slog.LevelError, []string{"ERROR"}},
	}
	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			var buf bytes.Buffer
			log := logger.NewWithWriter(&buf, tt.level)

			log.Debug("d")
			log.Info("i")
			log.Warn("w")
			log.Error("e")

			var got []string
			for _, rec := range parseLines(t, &buf) {
				got = append(got, rec["level"].(string))
			}
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("levels = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWithError_AddsErrorField(t *testing.T) {
	var buf bytes.Buffer
	log := logger.NewWithWriter(&buf, slog.LevelInfo)

	log.WithError(errors.New("boom")).Error("failed")

	recs := parseLines(t, &buf)
	if len(recs) != 1 {
		t.Fatalf("records = %d, want 1", len(recs))
	}
	if recs[0]["error"] != "boom" {
		t.Errorf("error field = %v, want boom", recs[0]["error"])
	}
}

func TestWithError_Nil(t *testing.T) {
	var buf bytes.Buffer
	log := logger.NewWithWriter(&buf, slog.LevelInfo)

	log.WithError(nil).Info("ok")

	recs := parseLines(t, &buf)
	if _, ok := recs[0]["error"]; ok {
		t.Errorf("unexpected error field for nil error: %v", recs[0])
	}
}

func TestWith_ChainDoesNotMutateParent(t *testing.T) {
	var buf bytes.Buffer
	parent := logger.NewWithWriter(&buf, slog.LevelInfo)

	child := parent.With("request_id", "r1")
	child.Info("child")
	parent.Info("parent")

	recs := parseLines(t, &buf)
	if recs[0]["request_id"] != "r1" {
		t.Errorf("child record lacks request_id: %v", recs[0])
	}
	if _, ok := recs[1]["request_id"]; ok {
		t.Errorf("parent record must not have request_id: %v", recs[1])
	}
}
