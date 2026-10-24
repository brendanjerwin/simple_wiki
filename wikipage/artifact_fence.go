package wikipage

import (
	"strings"
)

const (
	// ArtifactFenceInfoString is the info string used on the fenced code block
	// that holds a wiki artifact program. The fence delimiter plus this info
	// string uniquely identifies artifact source in a page body.
	ArtifactFenceInfoString = "wiki-artifact"

	artifactFenceOpen  = "```" + ArtifactFenceInfoString
	artifactFenceClose = "```"
)

// ArtifactFence holds the result of splitting a page body into its artifact
// program source and its prose remainder.
type ArtifactFence struct {
	// Source is the content inside the ```wiki-artifact fence (without the
	// fence delimiters). Empty when no fence was found.
	Source string
	// HasFence is true when a ```wiki-artifact fence was present in the body.
	HasFence bool
	// ProseRemainder is the full body text with the fence block removed.
	// When HasFence is false, ProseRemainder equals the original body.
	ProseRemainder string
}

// SplitArtifactFences parses body and separates the first ```wiki-artifact
// fenced block from the surrounding prose. It produces two independent
// outputs that must both be consumed:
//
//  1. ArtifactFence.Source — the program text inside the fence; routed to
//     the artifact validation funnel / GetArtifact; never rendered as prose.
//  2. ArtifactFence.ProseRemainder — body with the fence removed; what
//     goldmark renders, what Bleve indexes, what template executors process.
//
// Renderers and indexers should use ProseRemainder. The artifact host uses
// Source (via ArtifactService). Neither should see the other's input.
//
// If no fence is present, Source is empty, HasFence is false, and
// ProseRemainder equals body unchanged.
//
// Only the first fence is processed; subsequent ```wiki-artifact fences
// (which are invalid per the spec) are left in ProseRemainder and treated as
// ordinary fenced code blocks by downstream renderers.
func SplitArtifactFences(body string) ArtifactFence {
	openIdx := strings.Index(body, artifactFenceOpen)
	if openIdx == -1 {
		return ArtifactFence{ProseRemainder: body}
	}

	// Find the end of the opening fence line (info strings may have trailing
	// whitespace or options like `entry=index.html`; we accept any suffix).
	afterOpen := body[openIdx+len(artifactFenceOpen):]
	openLineEnd := strings.IndexByte(afterOpen, '\n')
	if openLineEnd == -1 {
		// Fence opened but never closed; treat body as-is.
		return ArtifactFence{ProseRemainder: body}
	}

	// Content starts after the newline that terminates the opening line.
	contentStart := openIdx + len(artifactFenceOpen) + openLineEnd + 1

	// Find the closing fence — a ``` on its own line. We search from
	// contentStart so that literal ``` inside the program text doesn't
	// prematurely terminate the fence.
	remainder := body[contentStart:]
	closeIdx := findClosingFence(remainder)
	if closeIdx == -1 {
		// Unclosed fence; treat body as-is to avoid silently discarding prose.
		return ArtifactFence{ProseRemainder: body}
	}

	source := remainder[:closeIdx]
	// closeIdx points to the start of the closing ``` line; skip past it.
	afterClose := closeIdx + len(artifactFenceClose)
	// Skip the newline following the closing fence, if present.
	if afterClose < len(remainder) && remainder[afterClose] == '\n' {
		afterClose++
	}
	afterFence := remainder[afterClose:]

	// Build prose remainder: everything before the fence + everything after.
	preBefore := strings.TrimRight(body[:openIdx], " \t")
	if preBefore != "" && !strings.HasSuffix(preBefore, newline) {
		preBefore += newline
	}
	// Trim the leading newline from afterFence to avoid a double blank line.
	prose := preBefore + strings.TrimLeft(afterFence, newline)

	return ArtifactFence{
		Source:         strings.TrimRight(source, newline),
		HasFence:       true,
		ProseRemainder: prose,
	}
}

// findClosingFence returns the index within s of the first line that starts
// with ``` (the closing fence delimiter). Returns -1 if not found.
func findClosingFence(s string) int {
	for idx := 0; idx < len(s); {
		end := strings.IndexByte(s[idx:], '\n')
		var line string
		if end == -1 {
			line = s[idx:]
		} else {
			line = s[idx : idx+end]
		}
		trimmed := strings.TrimRight(line, " \t\r")
		if trimmed == artifactFenceClose {
			return idx
		}
		if end == -1 {
			break
		}
		idx += end + 1
	}
	return -1
}
