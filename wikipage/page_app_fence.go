package wikipage

import (
	"strings"
)

// PageAppFenceInfo is the fence info-string that marks a page app program
// embedded in a page body. A body may carry at most one page app.
const PageAppFenceInfo = "wiki-page-app"

// PageAppFenceOpen is the exact fence line a page app program is wrapped in.
const PageAppFenceOpen = "```" + PageAppFenceInfo

// SplitPageAppFences separates a page app program from a page body.
//
// It returns (pageAppSource, proseRemainder, found):
//
//   - pageAppSource: the program text inside the fence (first fence wins;
//     subsequent fences are dropped, not rendered)
//   - proseRemainder: the body with ALL page app fences removed — this is what
//     goldmark renders, what search indexes, and what template executors
//     process (removal semantics, not bypass)
//   - found: whether a page app fence was present
//
// Fences are recognized by info-string exact match on "wiki-page-app" at line
// start; look-alikes ("wiki-page-apps") and ordinary code fences are left in
// the prose. An unterminated page app fence closes at end of body.
func SplitPageAppFences(body string) (source string, prose string, found bool) {
	var (
		sourceBuilder strings.Builder
		proseBuilder  strings.Builder
		fenceCount    int
	)
	inArtifact := false

	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimRight(line, " \t")
		switch {
		case !inArtifact && trimmed == PageAppFenceOpen:
			found = true
			fenceCount++
			inArtifact = true
		case inArtifact && trimmed == "```":
			inArtifact = false
		case inArtifact && fenceCount > 1:
			// A later page app fence: its content is never in prose and never
			// concatenated into the captured program (first fence wins).
		case inArtifact:
			sourceBuilder.WriteString(line)
			sourceBuilder.WriteByte('\n')
		default:
			proseBuilder.WriteString(line)
			proseBuilder.WriteByte('\n')
		}
	}

	// Rejoin without the trailing newline the split introduced per line, and
	// drop the page app source's final synthetic newline.
	source = strings.TrimSuffix(sourceBuilder.String(), "\n")

	return source, strings.TrimSuffix(proseBuilder.String(), "\n"), found
}
