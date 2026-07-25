package pagestore

import (
	"strings"
)

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

	// Compute the LCS table.
	lcs := computeLCSTable(oldLines, newLines)

	// Backtrack to produce the diff.
	type diffOp struct {
		op   byte // ' ', '-', '+'
		line string
	}
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

	// Group into hunks with 3 lines of context.
	const contextLines = 3
	var result strings.Builder
	idx := 0
	for idx < len(ops) {
		// Skip unchanged lines (context only).
		if ops[idx].op == ' ' {
			idx++
			continue
		}

		// Find the start of the hunk (include context lines before).
		hunkStart := idx
		for hunkStart > 0 && ops[hunkStart-1].op == ' ' && idx-hunkStart < contextLines {
			hunkStart--
		}

		// Find the end of the hunk.
		hunkEnd := idx
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

		// Trim trailing context to contextLines.
		for hunkEnd > hunkStart && ops[hunkEnd-1].op == ' ' && hunkEnd-1 > idx {
			hunkEnd--
		}

		// Count old and new line positions for the hunk header.
		oldStart, newStart := 1, 1
		for k := 0; k < hunkStart; k++ {
			if ops[k].op == ' ' || ops[k].op == '-' {
				oldStart++
			}
			if ops[k].op == ' ' || ops[k].op == '+' {
				newStart++
			}
		}

		oldLen, newLen := 0, 0
		for k := hunkStart; k < hunkEnd; k++ {
			switch ops[k].op {
			case ' ':
				oldLen++
				newLen++
			case '-':
				oldLen++
			case '+':
				newLen++
			}
		}

		// Write hunk header.
		result.WriteString(formatHunkHeader(oldStart, oldLen, newStart, newLen))
		result.WriteByte('\n')

		// Write hunk lines.
		for k := hunkStart; k < hunkEnd; k++ {
			result.WriteByte(ops[k].op)
			result.WriteString(ops[k].line)
			result.WriteByte('\n')
		}

		idx = hunkEnd
	}

	return result.String()
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
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
