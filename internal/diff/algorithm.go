package diff

import (
	"fmt"
	"strings"
)

// OpType represents the type of edit operation for a line.
type OpType int

const (
	// OpEqual indicates the line is unchanged.
	OpEqual OpType = iota
	// OpInsert indicates the line was added in the target version.
	OpInsert
	// OpDelete indicates the line was removed from the source version.
	OpDelete
)

// Edit represents a single line edit operation.
type Edit struct {
	Type OpType
	Text string
}

// ComputeLCS calculates the line-by-line diff between two text strings using the Longest Common Subsequence algorithm.
func ComputeLCS(aText, bText string) []Edit {
	aLines := splitLines(aText)
	bLines := splitLines(bText)

	n := len(aLines)
	m := len(bLines)

	// Build DP table for Longest Common Subsequence
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}

	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if aLines[i-1] == bLines[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				if dp[i-1][j] >= dp[i][j-1] {
					dp[i][j] = dp[i-1][j]
				} else {
					dp[i][j] = dp[i][j-1]
				}
			}
		}
	}

	// Backtrack to assemble edit script
	var edits []Edit
	i, j := n, m
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && aLines[i-1] == bLines[j-1] {
			edits = append(edits, Edit{Type: OpEqual, Text: aLines[i-1]})
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			edits = append(edits, Edit{Type: OpInsert, Text: bLines[j-1]})
			j--
		} else if i > 0 && (j == 0 || dp[i-1][j] >= dp[i][j-1]) {
			edits = append(edits, Edit{Type: OpDelete, Text: aLines[i-1]})
			i--
		}
	}

	// Reverse edits since we backtracked from end to beginning
	for k := 0; k < len(edits)/2; k++ {
		opp := len(edits) - 1 - k
		edits[k], edits[opp] = edits[opp], edits[k]
	}

	return edits
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	// Normalize CRLF to LF
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	// If text ends with a trailing newline, strings.Split produces a final empty element
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// UnifiedDiff formats line edits into standard unified diff format with header and chunk markers.
func UnifiedDiff(pathA, pathB string, aText, bText string) string {
	edits := ComputeLCS(aText, bText)
	if len(edits) == 0 {
		return ""
	}

	hasChanges := false
	for _, e := range edits {
		if e.Type != OpEqual {
			hasChanges = true
			break
		}
	}
	if !hasChanges {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- a/%s\n", pathA))
	sb.WriteString(fmt.Sprintf("+++ b/%s\n", pathB))

	// For simplicity and clarity in initial diffs, write a single contiguous unified hunk
	aCount := len(splitLines(aText))
	bCount := len(splitLines(bText))

	sb.WriteString(fmt.Sprintf("@@ -1,%d +1,%d @@\n", aCount, bCount))
	for _, edit := range edits {
		switch edit.Type {
		case OpEqual:
			sb.WriteString(fmt.Sprintf(" %s\n", edit.Text))
		case OpInsert:
			sb.WriteString(fmt.Sprintf("+%s\n", edit.Text))
		case OpDelete:
			sb.WriteString(fmt.Sprintf("-%s\n", edit.Text))
		}
	}

	return sb.String()
}
