package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	symlinkLine = regexp.MustCompile(`"([^"]+)"\s*:\s*"([^"]+)"`)
	arrayItem   = regexp.MustCompile(`"([^"]+)"`)
)

type MigrationResult struct {
	Symlinks  map[string]string
	Templates map[string]string
	Common    []string
	Darwin    []string
	LinuxAPT  []string
	LinuxBrew []string
	Ignore    []string
}

func MigrateTSConfig(tsPath string, tomlPath string) (MigrationResult, error) {
	file, err := os.Open(tsPath)
	if err != nil {
		return MigrationResult{}, err
	}
	defer file.Close()

	result := MigrationResult{
		Symlinks:  map[string]string{},
		Templates: map[string]string{},
		Common:    []string{},
		Darwin:    []string{},
		LinuxAPT:  []string{},
		LinuxBrew: []string{},
		Ignore:    []string{},
	}

	section := ""
	subsection := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case strings.HasPrefix(line, "symlinks:"):
			section = "symlinks"
			subsection = ""
		case strings.HasPrefix(line, "packages:"):
			section = "packages"
			subsection = ""
		case strings.HasPrefix(line, "templates:"):
			section = "templates"
			subsection = ""
		case strings.HasPrefix(line, "ignore:"):
			section = "ignore"
			subsection = ""
		case strings.HasPrefix(line, "common:"):
			subsection = "common"
		case strings.HasPrefix(line, "darwin:"):
			subsection = "darwin"
		case strings.HasPrefix(line, "linux:"):
			subsection = "linux"
		case strings.HasPrefix(line, "apt:"):
			if section == "packages" && subsection == "linux" {
				subsection = "linux_apt"
			}
		case strings.HasPrefix(line, "brew:"):
			if section == "packages" && (subsection == "linux" || subsection == "linux_apt") {
				subsection = "linux_brew"
			}
		}

		if section == "symlinks" || section == "templates" {
			if m := symlinkLine.FindStringSubmatch(line); len(m) == 3 {
				if section == "symlinks" {
					result.Symlinks[m[1]] = m[2]
				} else {
					result.Templates[m[1]] = m[2]
				}
			}
		}

		for _, m := range arrayItem.FindAllStringSubmatch(line, -1) {
			if len(m) != 2 {
				continue
			}
			item := m[1]
			switch subsection {
			case "common":
				result.Common = append(result.Common, item)
			case "darwin":
				result.Darwin = append(result.Darwin, item)
			case "linux_apt":
				result.LinuxAPT = append(result.LinuxAPT, item)
			case "linux_brew":
				result.LinuxBrew = append(result.LinuxBrew, item)
			}
			if section == "ignore" {
				result.Ignore = append(result.Ignore, item)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return result, err
	}

	builder := strings.Builder{}
	builder.WriteString("version = 1\n")
	builder.WriteString("layout = \"hybrid\"\n\n")
	builder.WriteString("[packages]\n")
	builder.WriteString(fmt.Sprintf("common = %s\n", tomlArray(result.Common)))
	builder.WriteString(fmt.Sprintf("darwin = %s\n", tomlArray(result.Darwin)))
	builder.WriteString(fmt.Sprintf("linux_apt = %s\n", tomlArray(result.LinuxAPT)))
	builder.WriteString(fmt.Sprintf("linux_brew = %s\n", tomlArray(result.LinuxBrew)))
	builder.WriteString("wsl_apt = []\n")
	builder.WriteString("wsl_brew = []\n\n")
	builder.WriteString("[hooks]\n\n")
	builder.WriteString("[ignore]\n")
	builder.WriteString(fmt.Sprintf("paths = %s\n\n", tomlArray(result.Ignore)))
	builder.WriteString("[backup]\n")
	builder.WriteString("enabled = true\nmax_age = 30\nmax_count = 5\n\n")
	if len(result.Symlinks) > 0 {
		builder.WriteString("[overrides]\n")
		for source, target := range result.Symlinks {
			builder.WriteString(fmt.Sprintf("\"%s\" = { target = \"%s\" }\n", source, target))
		}
		builder.WriteString("\n")
	}
	if len(result.Templates) > 0 {
		builder.WriteString("[templates]\n")
		for source, target := range result.Templates {
			builder.WriteString(fmt.Sprintf("\"%s\" = \"%s\"\n", source, target))
		}
	}

	if err := os.MkdirAll(filepath.Dir(tomlPath), 0o755); err != nil {
		return result, err
	}
	if err := os.WriteFile(tomlPath, []byte(builder.String()), 0o644); err != nil {
		return result, err
	}

	return result, nil
}

func tomlArray(items []string) string {
	quoted := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		quoted = append(quoted, fmt.Sprintf("\"%s\"", item))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
