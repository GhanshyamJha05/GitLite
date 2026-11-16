package lfs

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlite/internal/util"
)

const (
	// GitAttributesFile is the name of the attributes configuration file.
	GitAttributesFile = ".gitattributes"

	// LFSAttributeSuffix is standard git-lfs tracking attributes directive.
	LFSAttributeSuffix = "filter=lfs diff=lfs merge=lfs -text"
)

// Track adds a pattern to .gitattributes for LFS handling.
func Track(workTree, pattern string) error {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return fmt.Errorf("tracking pattern cannot be empty")
	}

	attrPath := filepath.Join(workTree, GitAttributesFile)
	patterns, _ := ListTracked(workTree)

	for _, p := range patterns {
		if p == pattern {
			// Already tracked
			return nil
		}
	}

	newLine := fmt.Sprintf("%s %s\n", pattern, LFSAttributeSuffix)

	var currentContent []byte
	if data, err := os.ReadFile(attrPath); err == nil {
		currentContent = data
		if len(currentContent) > 0 && !bytes.HasSuffix(currentContent, []byte("\n")) {
			currentContent = append(currentContent, '\n')
		}
	}

	currentContent = append(currentContent, []byte(newLine)...)
	return util.AtomicWriteFile(attrPath, currentContent, 0644)
}

// Untrack removes a pattern from .gitattributes.
func Untrack(workTree, pattern string) error {
	pattern = strings.TrimSpace(pattern)
	attrPath := filepath.Join(workTree, GitAttributesFile)

	data, err := os.ReadFile(attrPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	var newLines []string

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, pattern+" ") || trimmed == pattern {
			continue
		}
		newLines = append(newLines, line)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	out := strings.Join(newLines, "\n")
	if len(newLines) > 0 {
		out += "\n"
	}
	return util.AtomicWriteFile(attrPath, []byte(out), 0644)
}

// ListTracked returns all file patterns marked for Git LFS in .gitattributes.
func ListTracked(workTree string) ([]string, error) {
	attrPath := filepath.Join(workTree, GitAttributesFile)
	data, err := os.ReadFile(attrPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var patterns []string
	scanner := bufio.NewScanner(bytes.NewReader(data))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.Contains(line, "filter=lfs") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				patterns = append(patterns, parts[0])
			}
		}
	}

	return patterns, scanner.Err()
}

// MatchesPattern checks if path matches the given glob pattern.
func MatchesPattern(pattern, path string) bool {
	normPath := filepath.ToSlash(path)
	normPattern := filepath.ToSlash(pattern)

	// Direct match
	if matched, _ := filepath.Match(normPattern, normPath); matched {
		return true
	}
	// Basename match
	base := filepath.Base(normPath)
	if matched, _ := filepath.Match(normPattern, base); matched {
		return true
	}
	return false
}
