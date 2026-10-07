package pagestore

import (
	"strings"
)

// diffOp represents a single diff operation.
type diffOp struct {
	op   byte // ' ', '-', '+'
	line string
}

// computeUnifiedDiff produces a unified diff between two text snapshots.
// The diff is line-based, in the standard unified-diff format:
//
//	@@ -oldStart,oldLen +newStart,newLen @@
//	- removed line
//	+ added line
//	 context line
//
// This is a simple implementation using the classic LCS-based diff algorithm.
// Pages are small (KB scale), so the O(n*m) dynamic programming approach
// is adequate and avoids external dependencies.
func computeUnifiedDiff(oldText, newText string) string {
	oldLines := splitLines(oldText)
	newLines := splitLines(newText)
	lcs := computeLCSTable(oldLines, newLines)
	ops := backtrackDiff(lcs, oldLines, newLines)
	return formatDiffHunks(ops)
}

// backtrackDiff walks the LCS table backwards to produce a list of diff operations.
func backtrackDiff(lcs [][]int, oldLines, newLines []string) []diffOp {
	var ops []diffOp
	i, j := len(oldLines), len(newLines)
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			ops = append(ops, diffOp{op: ' ', line: oldLines[i-1]})
			i--
			j--
		} else if j > 0 && (i == 0 || lcs[i][j-1] >= lcs[i-1][j]) {
			ops = append(ops, diffOp{op: '+', line: newLines[j-1]})
			j--
		} else {
			ops = append(ops, diffOp{op: '-', line: oldLines[i-1]})
			i--
		}
	}
	// Reverse ops (we built them backwards).
	for left, right := 0, len(ops)-1; left < right; left, right = left+1, right-1 {
		ops[left], ops[right] = ops[right], ops[left]
	}
	return ops
}

// formatDiffHunks groups diff operations into hunks with context and formats them.
func formatDiffHunks(ops []diffOp) string {
	const contextLines = 3
	var result strings.Builder
	idx := 0
	for idx < len(ops) {
		if ops[idx].op == ' ' {
			idx++
			continue
		}
		hunkStart, hunkEnd := findHunkBounds(ops, idx, contextLines)
		c := countHunkCoords(ops, hunkStart, hunkEnd)
		result.WriteString(formatHunkHeader(c.oldStart, c.oldLen, c.newStart, c.newLen))
		result.WriteByte('\n')
		writeHunkLines(&result, ops, hunkStart, hunkEnd)
		idx = hunkEnd
	}
	return result.String()
}

// findHunkBounds returns the start and end indices for a hunk, including context lines.
func findHunkBounds(ops []diffOp, idx, contextLines int) (hunkStart, hunkEnd int) {
	hunkStart = idx
	for hunkStart > 0 && ops[hunkStart-1].op == ' ' && idx-hunkStart < contextLines {
		hunkStart--
	}
	hunkEnd = idx
	unchangedRun := 0
	for hunkEnd < len(ops) {
		if ops[hunkEnd].op == ' ' {
			unchangedRun++
			if unchangedRun > contextLines*2 {
				break
			}
		} else {
			unchangedRun = 0
		}
		hunkEnd++
	}
	for hunkEnd > hunkStart && ops[hunkEnd-1].op == ' ' && hunkEnd-1 > idx {
		hunkEnd--
	}
	return hunkStart, hunkEnd
}

// hunkCoords holds the position and length data needed for a unified diff hunk header.
type hunkCoords struct {
	oldStart, newStart int
	oldLen, newLen     int
}

// countHunkCoords computes the old/new line start positions and lengths for a hunk header.
func countHunkCoords(ops []diffOp, hunkStart, hunkEnd int) hunkCoords {
	c := hunkCoords{oldStart: 1, newStart: 1}
	for k := 0; k < hunkStart; k++ {
		if ops[k].op == ' ' || ops[k].op == '-' {
			c.oldStart++
		}
		if ops[k].op == ' ' || ops[k].op == '+' {
			c.newStart++
		}
	}
	for k := hunkStart; k < hunkEnd; k++ {
		switch ops[k].op {
		case ' ':
			c.oldLen++
			c.newLen++
		case '-':
			c.oldLen++
		case '+':
			c.newLen++
		default:
			// unreachable: op is always one of ' ', '-', '+'
		}
	}
	return c
}

// writeHunkLines writes the diff lines for a hunk to the builder.
func writeHunkLines(result *strings.Builder, ops []diffOp, hunkStart, hunkEnd int) {
	for k := hunkStart; k < hunkEnd; k++ {
		result.WriteByte(ops[k].op)
		result.WriteString(ops[k].line)
		result.WriteByte('\n')
	}
}

// splitLines splits text into lines, preserving the content without trailing newlines.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	// Remove trailing empty element from trailing newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// computeLCSTable computes the dynamic programming table for the longest
// common subsequence of two line slices. Returns a (len(a)+1) x (len(b)+1)
// table where lcs[i][j] is the LCS length of a[:i] and b[:j].
func computeLCSTable(a, b []string) [][]int {
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}

	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				lcs[i][j] = lcs[i-1][j-1] + 1
			} else if lcs[i-1][j] >= lcs[i][j-1] {
				lcs[i][j] = lcs[i-1][j]
			} else {
				lcs[i][j] = lcs[i][j-1]
			}
		}
	}

	return lcs
}

// formatHunkHeader produces the @@ -oldStart,oldLen +newStart,newLen @@ header.
func formatHunkHeader(oldStart, oldLen, newStart, newLen int) string {
	return "@@ " + formatRange('-', oldStart, oldLen) + " " + formatRange('+', newStart, newLen) + " @@"
}

func formatRange(prefix byte, start, length int) string {
	if length == 1 {
		return string(prefix) + itoa(start)
	}
	return string(prefix) + itoa(start) + "," + itoa(length)
}

// itoa is a minimal int-to-string to avoid importing strconv for one use.
func itoa(n int) string {
	const decimalBase = 10
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%decimalBase)
		n /= decimalBase
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
