package output

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// Logger writes text or JSON logs.
type Logger struct {
	format  string
	quiet   bool
	verbose bool
	mu      sync.Mutex
}

func NewLogger(format string, quiet bool, verbose bool) *Logger {
	f := strings.ToLower(strings.TrimSpace(format))
	if f == "" {
		f = "text"
	}
	if f != "json" {
		f = "text"
	}
	return &Logger{format: f, quiet: quiet, verbose: verbose}
}

func (l *Logger) Header(msg string) {
	l.log("header", msg, nil)
}

func (l *Logger) Subheader(msg string) {
	l.log("subheader", msg, nil)
}

func (l *Logger) Info(msg string) {
	l.log("info", msg, nil)
}

func (l *Logger) Warn(msg string) {
	l.log("warn", msg, nil)
}

func (l *Logger) Error(msg string) {
	l.log("error", msg, nil)
}

func (l *Logger) Success(msg string) {
	l.log("success", msg, nil)
}

func (l *Logger) DryRun(msg string) {
	l.log("dryrun", msg, nil)
}

func (l *Logger) Debug(msg string) {
	if !l.verbose {
		return
	}
	l.log("debug", msg, nil)
}

func (l *Logger) Table(values map[string]string) {
	if len(values) == 0 {
		return
	}
	if l.format == "json" {
		l.log("table", "table", map[string]any{"values": values})
		return
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(os.Stdout, "  %-16s %s\n", key, values[key])
	}
}

func (l *Logger) log(level string, msg string, fields map[string]any) {
	if l.quiet && level != "error" && level != "warn" {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.format == "json" {
		entry := map[string]any{
			"ts":    time.Now().UTC().Format(time.RFC3339),
			"level": level,
			"msg":   msg,
		}
		for k, v := range fields {
			entry[k] = v
		}
		_ = json.NewEncoder(os.Stdout).Encode(entry)
		return
	}

	prefix := map[string]string{
		"header":    "===",
		"subheader": "---",
		"info":      "i",
		"warn":      "!",
		"error":     "x",
		"success":   "+",
		"dryrun":    "o",
		"debug":     "d",
		"table":     "t",
	}[level]
	if prefix == "" {
		prefix = "-"
	}
	fmt.Fprintf(os.Stdout, "%s %s\n", prefix, msg)
}
