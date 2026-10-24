package wikipage

import (
	"strings"
)

// ArtifactFenceInfo is the fence info-string that marks an artifact program
// embedded in a page body. A body may carry at most one artifact.
const ArtifactFenceInfo = "wiki-artifact"

// ArtifactFenceOpen is the exact fence line an artifact program is wrapped in.
const ArtifactFenceOpen = "```" + ArtifactFenceInfo

// SplitArtifactFences separates an artifact program from a page body.
//
// It returns (artifactSource, proseRemainder, found):
//
//   - artifactSource: the program text inside the fence (first fence wins;
//     subsequent fences are dropped, not rendered)
//   - proseRemainder: the body with ALL artifact fences removed — this is what
//     goldmark renders, what search indexes, and what template executors
//     process (removal semantics, not bypass)
//   - found: whether an artifact fence was present
//
// Fences are recognized by info-string exact match on "wiki-artifact" at line
// start; look-alikes ("wiki-artifacts") and ordinary code fences are left in
// the prose. An unterminated artifact fence closes at end of body.
func SplitArtifactFences(body string) (source string, prose string, found bool) {
	var (
		sourceBuilder strings.Builder
		proseBuilder  strings.Builder
		fenceCount    int
	)
	inArtifact := false

	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimRight(line, " \t")
		switch {
		case !inArtifact && trimmed == ArtifactFenceOpen:
			found = true
			fenceCount++
			inArtifact = true
		case inArtifact && trimmed == "```":
			inArtifact = false
		case inArtifact && fenceCount > 1:
			// A later artifact fence: its content is never in prose and never
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
	// drop the artifact source's final synthetic newline.
	source = strings.TrimSuffix(sourceBuilder.String(), "\n")

	return source, strings.TrimSuffix(proseBuilder.String(), "\n"), found
}
