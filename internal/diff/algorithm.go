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

// SplitLines splits a string into lines, normalizing CRLF to LF and stripping a trailing empty line.
func SplitLines(s string) []string {
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

func splitLines(s string) []string {
	return SplitLines(s)
}

// Hunk represents a cohesive block of diff edits with line numbers.
type Hunk struct {
	AStart int
	ACount int
	BStart int
	BCount int
	Edits  []Edit
}

// IsBinary checks whether a byte sequence represents binary data rather than text.
// It handles UTF-16 BOMs (LE / BE) and UTF-8 BOM, and distinguishes encoded Unicode text from binary data.
func IsBinary(s string) bool {
	data := []byte(s)
	limit := len(data)
	if limit == 0 {
		return false
	}

	// 1. Check for standard UTF-16 BOMs
	// UTF-16 Little Endian: 0xFF, 0xFE
	// UTF-16 Big Endian: 0xFE, 0xFF
	if limit >= 2 {
		if (data[0] == 0xFF && data[1] == 0xFE) || (data[0] == 0xFE && data[1] == 0xFF) {
			return false
		}
	}

	// 2. Check for UTF-8 BOM: 0xEF, 0xBB, 0xBF
	if limit >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return false
	}

	if limit > 8000 {
		limit = 8000
	}

	// 3. Heuristic for UTF-16 without BOM (common on Windows PowerShell / registry exports / notepad):
	// In UTF-16 text of ASCII chars, alternating bytes are null (0x00).
	// If nulls appear in alternating even or odd positions with valid printable/whitespace ASCII on the other, it's UTF-16.
	nullCount := 0
	evenNulls := 0
	oddNulls := 0
	for i := 0; i < limit; i++ {
		if data[i] == 0 {
			nullCount++
			if i%2 == 0 {
				evenNulls++
			} else {
				oddNulls++
			}
		}
	}

	if nullCount == 0 {
		return false
	}

	// If roughly half the bytes are null and they are concentrated on even or odd offsets, treat as UTF-16 text
	pairs := limit / 2
	if pairs > 2 && (oddNulls >= pairs*4/5 || evenNulls >= pairs*4/5) {
		return false
	}

	return true
}

// UnifiedDiff formats line edits into standard unified diff format with header and per-hunk line ranges.
func UnifiedDiff(pathA, pathB string, aText, bText string) string {
	if aText == bText {
		return ""
	}

	if IsBinary(aText) || IsBinary(bText) {
		return fmt.Sprintf("Binary files a/%s and b/%s differ\n", pathA, pathB)
	}

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

	hunks := groupHunks(edits, 3)
	if len(hunks) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- a/%s\n", pathA))
	sb.WriteString(fmt.Sprintf("+++ b/%s\n", pathB))

	hasNewlineA := strings.HasSuffix(aText, "\n") || strings.HasSuffix(aText, "\r")
	hasNewlineB := strings.HasSuffix(bText, "\n") || strings.HasSuffix(bText, "\r")

	for i, h := range hunks {
		sb.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", h.AStart, h.ACount, h.BStart, h.BCount))
		isLastHunk := (i == len(hunks)-1)
		for editIdx, edit := range h.Edits {
			switch edit.Type {
			case OpEqual:
				sb.WriteString(fmt.Sprintf(" %s\n", edit.Text))
				if isLastHunk && editIdx == len(h.Edits)-1 {
					if !hasNewlineA && !hasNewlineB {
						sb.WriteString("\\ No newline at end of file\n")
					}
				}
			case OpInsert:
				sb.WriteString(fmt.Sprintf("+%s\n", edit.Text))
				if isLastHunk && editIdx == len(h.Edits)-1 && !hasNewlineB {
					sb.WriteString("\\ No newline at end of file\n")
				}
			case OpDelete:
				sb.WriteString(fmt.Sprintf("-%s\n", edit.Text))
				if isLastHunk && editIdx == len(h.Edits)-1 && !hasNewlineA {
					sb.WriteString("\\ No newline at end of file\n")
				}
			}
		}
	}

	return sb.String()
}

type diffOp struct {
	edit Edit
	aIdx int // 1-based line number in A (0 if insertion)
	bIdx int // 1-based line number in B (0 if deletion)
}

// groupHunks groups diff edits into separate hunks separated by unchanged context lines.
func groupHunks(edits []Edit, contextLines int) []Hunk {
	var ops []diffOp
	aNum := 1
	bNum := 1

	for _, e := range edits {
		op := diffOp{edit: e}
		switch e.Type {
		case OpEqual:
			op.aIdx = aNum
			op.bIdx = bNum
			aNum++
			bNum++
		case OpDelete:
			op.aIdx = aNum
			aNum++
		case OpInsert:
			op.bIdx = bNum
			bNum++
		}
		ops = append(ops, op)
	}

	// Identify indices of modified operations (inserts and deletes)
	var changeIndices []int
	for idx, op := range ops {
		if op.edit.Type != OpEqual {
			changeIndices = append(changeIndices, idx)
		}
	}

	if len(changeIndices) == 0 {
		return nil
	}

	var hunks []Hunk
	startChange := changeIndices[0]
	endChange := changeIndices[0]

	for i := 1; i < len(changeIndices); i++ {
		idx := changeIndices[i]
		// Distance in equal lines between changes
		if idx-endChange <= 2*contextLines+1 {
			endChange = idx
		} else {
			// Flush current hunk
			h := buildHunk(ops, startChange, endChange, contextLines)
			hunks = append(hunks, h)
			startChange = idx
			endChange = idx
		}
	}
	// Flush last hunk
	h := buildHunk(ops, startChange, endChange, contextLines)
	hunks = append(hunks, h)

	return hunks
}

func buildHunk(ops []diffOp, firstChange, lastChange, contextLines int) Hunk {
	start := firstChange - contextLines
	if start < 0 {
		start = 0
	}
	end := lastChange + contextLines + 1
	if end > len(ops) {
		end = len(ops)
	}

	slice := ops[start:end]
	var edits []Edit
	aCount := 0
	bCount := 0
	aStart := 0
	bStart := 0

	for _, op := range slice {
		edits = append(edits, op.edit)
		if op.edit.Type == OpEqual || op.edit.Type == OpDelete {
			if aStart == 0 {
				aStart = op.aIdx
			}
			aCount++
		}
		if op.edit.Type == OpEqual || op.edit.Type == OpInsert {
			if bStart == 0 {
				bStart = op.bIdx
			}
			bCount++
		}
	}

	if aStart == 0 {
		aStart = 1
	}
	if bStart == 0 {
		bStart = 1
	}

	return Hunk{
		AStart: aStart,
		ACount: aCount,
		BStart: bStart,
		BCount: bCount,
		Edits:  edits,
	}
}
