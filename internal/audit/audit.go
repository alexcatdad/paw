package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/platform"
)

type Severity string

const (
	SeverityError      Severity = "error"
	SeverityWarning    Severity = "warning"
	SeverityInfo       Severity = "info"
	SeveritySuggestion Severity = "suggestion"
)

type Finding struct {
	Severity   Severity `json:"severity"`
	Category   string   `json:"category"`
	Message    string   `json:"message"`
	Path       string   `json:"path,omitempty"`
	Suggestion string   `json:"suggestion,omitempty"`
}

type Result struct {
	Timestamp string    `json:"timestamp"`
	RepoPath  string    `json:"repoPath"`
	Findings  []Finding `json:"findings"`
	Summary   struct {
		Errors      int `json:"errors"`
		Warnings    int `json:"warnings"`
		Info        int `json:"info"`
		Suggestions int `json:"suggestions"`
	} `json:"summary"`
	Score int `json:"score"`
}

type Options struct {
	JSON        bool
	Verbose     bool
	MinSeverity Severity
}

func Run(repoPath string, cfg *config.Config) (Result, error) {
	files := []string{}
	_ = filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(repoPath, path)
		if err != nil || rel == "." {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "dist" || d.Name() == "build" {
				return filepath.SkipDir
			}
			return nil
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})

	findings := []Finding{}
	if _, err := os.Stat(filepath.Join(repoPath, "paw.toml")); err != nil {
		findings = append(findings, Finding{Severity: SeverityError, Category: "structure", Message: "Missing paw.toml", Path: "paw.toml", Suggestion: "Create paw.toml"})
	} else {
		findings = append(findings, Finding{Severity: SeverityInfo, Category: "structure", Message: "Found paw.toml", Path: "paw.toml"})
	}

	if hasMixedNaming(files) {
		findings = append(findings, Finding{Severity: SeverityWarning, Category: "naming", Message: "Mixed naming conventions detected"})
	} else {
		findings = append(findings, Finding{Severity: SeverityInfo, Category: "naming", Message: "Naming convention is consistent"})
	}

	hasHome := hasPrefix(files, "home/")
	if !hasHome {
		findings = append(findings, Finding{Severity: SeverityWarning, Category: "structure", Message: "Missing home/ directory for hybrid layout", Suggestion: "Move managed files under home/"})
	} else {
		findings = append(findings, Finding{Severity: SeverityInfo, Category: "structure", Message: "Using hybrid home/ layout"})
	}

	if cfg != nil {
		if len(cfg.Packages.Common) == 0 {
			findings = append(findings, Finding{Severity: SeveritySuggestion, Category: "missing", Message: "No common packages defined"})
		}
		if platform.Current() == platform.Darwin && len(cfg.Packages.Darwin) == 0 {
			findings = append(findings, Finding{Severity: SeverityInfo, Category: "missing", Message: "No darwin packages configured"})
		}
	}

	result := Result{Timestamp: time.Now().UTC().Format(time.RFC3339), RepoPath: repoPath, Findings: findings}
	for _, f := range findings {
		switch f.Severity {
		case SeverityError:
			result.Summary.Errors++
		case SeverityWarning:
			result.Summary.Warnings++
		case SeveritySuggestion:
			result.Summary.Suggestions++
		default:
			result.Summary.Info++
		}
	}
	score := 100 - (result.Summary.Errors * 20) - (result.Summary.Warnings * 10) - (result.Summary.Suggestions * 3)
	if score < 0 {
		score = 0
	}
	result.Score = score
	return result, nil
}

func Print(res Result, opts Options, logger *output.Logger) {
	if opts.JSON {
		payload, _ := json.MarshalIndent(res, "", "  ")
		logger.Info(string(payload))
		return
	}
	logger.Header("Dotfiles Audit")
	logger.Table(map[string]string{"Repository": res.RepoPath, "Score": fmtScore(res.Score)})
	for _, f := range res.Findings {
		if !severityAllowed(f.Severity, opts.MinSeverity) {
			continue
		}
		switch f.Severity {
		case SeverityError:
			logger.Error(f.Message)
		case SeverityWarning:
			logger.Warn(f.Message)
		case SeveritySuggestion:
			logger.Info(f.Message)
		default:
			logger.Success(f.Message)
		}
		if opts.Verbose && strings.TrimSpace(f.Suggestion) != "" {
			logger.Info("  " + f.Suggestion)
		}
	}
}

func hasPrefix(files []string, prefix string) bool {
	for _, file := range files {
		if strings.HasPrefix(file, prefix) {
			return true
		}
	}
	return false
}

func hasMixedNaming(files []string) bool {
	dot := 0
	nodot := 0
	for _, file := range files {
		if strings.HasPrefix(filepath.Base(file), ".") {
			dot++
		} else if !strings.Contains(file, "/") {
			nodot++
		}
	}
	return dot > 0 && nodot > 0
}

func severityAllowed(current Severity, min Severity) bool {
	if min == "" {
		return true
	}
	order := map[Severity]int{SeverityError: 0, SeverityWarning: 1, SeveritySuggestion: 2, SeverityInfo: 3}
	return order[current] <= order[min]
}

func fmtScore(score int) string {
	return fmt.Sprintf("%d/100", score)
}
