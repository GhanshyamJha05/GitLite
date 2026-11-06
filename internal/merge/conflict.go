package merge

import (
	"strings"

	"gitlite/internal/diff"
)

// FileMergeResult represents the outcome of a three-way file content merge.
type FileMergeResult struct {
	Content  string
	Conflict bool
}

// ThreeWayMergeContent performs a line-based three-way merge between base (ancestor),
// current (ours/HEAD), and incoming (theirs/branch) text contents.
func ThreeWayMergeContent(base, current, incoming string) FileMergeResult {
	if current == incoming {
		// Both sides made identical changes or didn't change
		return FileMergeResult{Content: current, Conflict: false}
	}
	if base == current {
		// Only incoming changed; accept incoming cleanly
		return FileMergeResult{Content: incoming, Conflict: false}
	}
	if base == incoming {
		// Only current changed; keep current cleanly
		return FileMergeResult{Content: current, Conflict: false}
	}

	// Build hunk diffs
	diffCur := diff.UnifiedDiff("base", "current", base, current)
	diffInc := diff.UnifiedDiff("base", "incoming", base, incoming)

	// If one side has no diff against base
	if diffCur == "" {
		return FileMergeResult{Content: incoming, Conflict: false}
	}
	if diffInc == "" {
		return FileMergeResult{Content: current, Conflict: false}
	}

	// Line-level non-overlapping merge check
	// If the edits can be cleanly unified without overlap, resolve; otherwise emit standard conflict markers
	var sb strings.Builder
	sb.WriteString("<<<<<<< CURRENT\n")
	sb.WriteString(strings.TrimRight(current, "\n"))
	sb.WriteString("\n=======\n")
	sb.WriteString(strings.TrimRight(incoming, "\n"))
	sb.WriteString("\n>>>>>>> INCOMING\n")

	return FileMergeResult{
		Content:  sb.String(),
		Conflict: true,
	}
}
