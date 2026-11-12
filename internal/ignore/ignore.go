package ignore

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Pattern represents a single compiled ignore rule.
type Pattern struct {
	Raw        string
	Negated    bool
	DirOnly    bool
	Rooted     bool
	Pattern    string
	BaseRelDir string // relative directory where this pattern was defined (e.g. "" for root)
}

// Matcher evaluates file paths against a collection of ignore patterns.
type Matcher struct {
	patterns []Pattern
}

// NewMatcher creates an empty Matcher.
func NewMatcher() *Matcher {
	return &Matcher{
		patterns: make([]Pattern, 0),
	}
}

// LoadFile reads an ignore file (e.g. .gitliteignore or .gitignore) and appends its patterns.
func (m *Matcher) LoadFile(path, baseRelDir string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		m.AddLine(line, baseRelDir)
	}

	return scanner.Err()
}

// AddLine parses and appends a single line from an ignore file.
func (m *Matcher) AddLine(line, baseRelDir string) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return
	}

	negated := false
	if strings.HasPrefix(line, "!") {
		negated = true
		line = strings.TrimPrefix(line, "!")
	}

	dirOnly := false
	if strings.HasSuffix(line, "/") {
		dirOnly = true
		line = strings.TrimSuffix(line, "/")
	}

	rooted := false
	if strings.HasPrefix(line, "/") {
		rooted = true
		line = strings.TrimPrefix(line, "/")
	} else if strings.Contains(line, "/") {
		// If pattern contains a slash inside, it is rooted relative to the pattern file location
		rooted = true
	}

	baseRelDir = filepath.ToSlash(baseRelDir)
	baseRelDir = strings.TrimPrefix(baseRelDir, "/")
	baseRelDir = strings.TrimSuffix(baseRelDir, "/")

	m.patterns = append(m.patterns, Pattern{
		Raw:        line,
		Negated:    negated,
		DirOnly:    dirOnly,
		Rooted:     rooted,
		Pattern:    line,
		BaseRelDir: baseRelDir,
	})
}

// Matches checks whether relPath (relative to repository root) matches any ignore pattern.
// isDir indicates whether the evaluated path is a directory.
func (m *Matcher) Matches(relPath string, isDir bool) bool {
	norm := filepath.ToSlash(relPath)
	norm = strings.TrimPrefix(norm, "./")
	norm = strings.TrimPrefix(norm, "/")

	ignored := false

	for _, p := range m.patterns {
		if p.DirOnly && !isDir && !strings.Contains(norm, "/") {
			// If path is a file at the root, a DirOnly pattern cannot match it
			continue
		}

		if matchPattern(p, norm, isDir) {
			ignored = !p.Negated
		}
	}

	return ignored
}

func matchPattern(p Pattern, path string, isDir bool) bool {
	target := path
	if p.BaseRelDir != "" {
		if !strings.HasPrefix(target, p.BaseRelDir+"/") && target != p.BaseRelDir {
			return false
		}
		target = strings.TrimPrefix(target, p.BaseRelDir+"/")
	}

	if p.Rooted {
		matched, err := filepath.Match(p.Pattern, target)
		if err == nil && matched {
			if !p.DirOnly || isDir {
				return true
			}
		}
		// Also match directory prefix if pattern points to a directory (e.g. "build/" matches "build/output.bin")
		if strings.HasPrefix(target, p.Pattern+"/") {
			return true
		}
		return false
	}

	// Unrooted pattern: matches any path component or file name
	if !p.DirOnly || isDir {
		baseName := filepath.Base(target)
		matched, err := filepath.Match(p.Pattern, baseName)
		if err == nil && matched {
			return true
		}
	}

	// Match any directory in the path hierarchy (e.g. "build" matching "foo/build/bar.txt")
	parts := strings.Split(target, "/")
	// If path is a file, the last part is not a directory
	limit := len(parts)
	if !isDir {
		limit--
	}
	for i := 0; i < limit; i++ {
		if m, err := filepath.Match(p.Pattern, parts[i]); err == nil && m {
			return true
		}
	}

	return false
}
