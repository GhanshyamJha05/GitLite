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

// LineChange represents modifications applied to a slice of the base text.
type LineChange struct {
	BaseStart int      // 0-indexed start line in base
	BaseEnd   int      // 0-indexed exclusive end line in base
	NewLines  []string // Replacement lines
}

// ThreeWayMergeContent performs a line-based three-way merge between base (ancestor),
// current (ours/HEAD), and incoming (theirs/branch) text contents.
// It detects and applies non-overlapping changes from both sides cleanly, and emits
// standard conflict markers only when edits overlap on the same base lines.
func ThreeWayMergeContent(base, current, incoming string) FileMergeResult {
	if current == incoming {
		return FileMergeResult{Content: current, Conflict: false}
	}
	if base == current {
		return FileMergeResult{Content: incoming, Conflict: false}
	}
	if base == incoming {
		return FileMergeResult{Content: current, Conflict: false}
	}

	// Binary check: If any side contains null bytes, line-by-line merge cannot be performed safely
	if diff.IsBinary(base) || diff.IsBinary(current) || diff.IsBinary(incoming) {
		return FileMergeResult{
			Content:  current,
			Conflict: true,
		}
	}

	baseLines := diff.SplitLines(base)
	curEdits := diff.ComputeLCS(base, current)
	incEdits := diff.ComputeLCS(base, incoming)

	curChanges := extractChanges(curEdits)
	incChanges := extractChanges(incEdits)

	return mergeChanges(baseLines, curChanges, incChanges)
}

func extractChanges(edits []diff.Edit) []LineChange {
	var changes []LineChange
	baseIdx := 0

	i := 0
	for i < len(edits) {
		if edits[i].Type == diff.OpEqual {
			baseIdx++
			i++
			continue
		}

		startBase := baseIdx
		var newLines []string

		for i < len(edits) && edits[i].Type != diff.OpEqual {
			switch edits[i].Type {
			case diff.OpDelete:
				baseIdx++
			case diff.OpInsert:
				newLines = append(newLines, edits[i].Text)
			}
			i++
		}

		changes = append(changes, LineChange{
			BaseStart: startBase,
			BaseEnd:   baseIdx,
			NewLines:  newLines,
		})
	}

	return changes
}

func mergeChanges(baseLines []string, curChanges, incChanges []LineChange) FileMergeResult {
	var resultLines []string
	hasConflict := false

	baseIdx := 0
	curI, incI := 0, 0

	for baseIdx < len(baseLines) || curI < len(curChanges) || incI < len(incChanges) {
		nextCur := curI < len(curChanges)
		nextInc := incI < len(incChanges)

		if !nextCur && !nextInc {
			// No more changes on either side; append remaining base lines
			resultLines = append(resultLines, baseLines[baseIdx:]...)
			break
		}

		// Find the next change to process
		var curCh, incCh *LineChange
		if nextCur {
			curCh = &curChanges[curI]
		}
		if nextInc {
			incCh = &incChanges[incI]
		}

		// Check if both sides have changes at or before current baseIdx
		if nextCur && nextInc && changesOverlap(*curCh, *incCh) {
			// Group all transitively overlapping changes
			overlapCur, overlapInc, nextCurI, nextIncI := collectOverlappingGroup(curChanges, incChanges, curI, incI)

			groupStart := overlapCur[0].BaseStart
			if overlapInc[0].BaseStart < groupStart {
				groupStart = overlapInc[0].BaseStart
			}
			groupEnd := overlapCur[len(overlapCur)-1].BaseEnd
			if overlapInc[len(overlapInc)-1].BaseEnd > groupEnd {
				groupEnd = overlapInc[len(overlapInc)-1].BaseEnd
			}

			// Add base lines up to groupStart
			if baseIdx < groupStart {
				resultLines = append(resultLines, baseLines[baseIdx:groupStart]...)
			}

			// Reconstruct side contents for the region [groupStart, groupEnd]
			curSlice := reconstructRegion(baseLines, groupStart, groupEnd, overlapCur)
			incSlice := reconstructRegion(baseLines, groupStart, groupEnd, overlapInc)

			if slicesEqual(curSlice, incSlice) {
				// Both sides made identical changes in this region
				resultLines = append(resultLines, curSlice...)
			} else {
				// Overlapping conflicting change!
				hasConflict = true
				resultLines = append(resultLines, "<<<<<<< CURRENT")
				resultLines = append(resultLines, curSlice...)
				resultLines = append(resultLines, "=======")
				resultLines = append(resultLines, incSlice...)
				resultLines = append(resultLines, ">>>>>>> INCOMING")
			}

			baseIdx = groupEnd
			curI = nextCurI
			incI = nextIncI
			continue
		}

		// Non-overlapping changes: decide which one comes first
		curEarlier := nextCur && (!nextInc || curCh.BaseStart < incCh.BaseStart || (curCh.BaseStart == incCh.BaseStart && curCh.BaseEnd < incCh.BaseEnd))

		if curEarlier {
			// Current change comes first without overlap
			if baseIdx < curCh.BaseStart {
				resultLines = append(resultLines, baseLines[baseIdx:curCh.BaseStart]...)
			}
			resultLines = append(resultLines, curCh.NewLines...)
			baseIdx = curCh.BaseEnd
			curI++
		} else {
			// Incoming change comes first without overlap
			if baseIdx < incCh.BaseStart {
				resultLines = append(resultLines, baseLines[baseIdx:incCh.BaseStart]...)
			}
			resultLines = append(resultLines, incCh.NewLines...)
			baseIdx = incCh.BaseEnd
			incI++
		}
	}

	resContent := ""
	if len(resultLines) > 0 {
		resContent = strings.Join(resultLines, "\n") + "\n"
	}

	return FileMergeResult{
		Content:  resContent,
		Conflict: hasConflict,
	}
}

func changesOverlap(a, b LineChange) bool {
	// If both are pure insertions at the same position, they overlap
	if a.BaseStart == a.BaseEnd && b.BaseStart == b.BaseEnd && a.BaseStart == b.BaseStart {
		return true
	}
	// General interval overlap: max(start) < min(end)
	// Or an insertion inside an edit interval: start <= ins < end
	if a.BaseStart == a.BaseEnd {
		return a.BaseStart >= b.BaseStart && a.BaseStart < b.BaseEnd
	}
	if b.BaseStart == b.BaseEnd {
		return b.BaseStart >= a.BaseStart && b.BaseStart < a.BaseEnd
	}
	return a.BaseStart < b.BaseEnd && b.BaseStart < a.BaseEnd
}

func collectOverlappingGroup(curChanges, incChanges []LineChange, startCurI, startIncI int) ([]LineChange, []LineChange, int, int) {
	groupCur := []LineChange{curChanges[startCurI]}
	groupInc := []LineChange{incChanges[startIncI]}
	curI := startCurI + 1
	incI := startIncI + 1

	for {
		changed := false

		// Determine current group bounds
		gStart := groupCur[0].BaseStart
		if groupInc[0].BaseStart < gStart {
			gStart = groupInc[0].BaseStart
		}
		gEnd := groupCur[0].BaseEnd
		for _, c := range groupCur {
			if c.BaseStart < gStart {
				gStart = c.BaseStart
			}
			if c.BaseEnd > gEnd {
				gEnd = c.BaseEnd
			}
		}
		for _, c := range groupInc {
			if c.BaseStart < gStart {
				gStart = c.BaseStart
			}
			if c.BaseEnd > gEnd {
				gEnd = c.BaseEnd
			}
		}

		dummy := LineChange{BaseStart: gStart, BaseEnd: gEnd}

		if curI < len(curChanges) && changesOverlap(dummy, curChanges[curI]) {
			groupCur = append(groupCur, curChanges[curI])
			curI++
			changed = true
		}
		if incI < len(incChanges) && changesOverlap(dummy, incChanges[incI]) {
			groupInc = append(groupInc, incChanges[incI])
			incI++
			changed = true
		}

		if !changed {
			break
		}
	}

	return groupCur, groupInc, curI, incI
}

func reconstructRegion(baseLines []string, regionStart, regionEnd int, changes []LineChange) []string {
	var lines []string
	curBase := regionStart

	for _, ch := range changes {
		if curBase < ch.BaseStart {
			lines = append(lines, baseLines[curBase:ch.BaseStart]...)
		}
		lines = append(lines, ch.NewLines...)
		curBase = ch.BaseEnd
	}

	if curBase < regionEnd {
		lines = append(lines, baseLines[curBase:regionEnd]...)
	}

	return lines
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
