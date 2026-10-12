package goldmarkrenderer

import (
	"bufio"
	"bytes"
	"errors"
	"testing"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// errSimulatedWrite is returned by failingWriter once its budget of
// successful write operations is exhausted.
var errSimulatedWrite = errors.New("simulated write failure")

const (
	testExtURL   = "https://example.com"
	testEmail    = "user@example.com"
	testAttrName = "rel"
	testAttrVal  = "noopener"
	sweepBudget  = 12
)

// failingWriter implements util.BufWriter and fails the Nth write operation
// (budget counts successful operations first). Sweeping the budget across a
// range walks every error-return branch in the renderer under test.
type failingWriter struct {
	budget int
}

// compile-time check that failingWriter satisfies the renderer's writer interface.
var _ util.BufWriter = (*failingWriter)(nil)

func (f *failingWriter) spend() error {
	if f.budget <= 0 {
		return errSimulatedWrite
	}
	f.budget--
	return nil
}

func (f *failingWriter) Write(p []byte) (int, error) {
	if err := f.spend(); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (f *failingWriter) WriteString(s string) (int, error) {
	if err := f.spend(); err != nil {
		return 0, err
	}
	return len(s), nil
}

func (f *failingWriter) WriteByte(byte) error { return f.spend() }

func (f *failingWriter) WriteRune(r rune) (int, error) {
	if err := f.spend(); err != nil {
		return 0, err
	}
	return len(string(r)), nil
}

func (*failingWriter) Available() int { return 0 }

func (*failingWriter) Buffered() int { return 0 }

func (*failingWriter) Flush() error { return nil }

func newTestExternalLinkRenderer(t *testing.T) *externalLinkRenderer {
	t.Helper()
	r, ok := NewExternalLinkRenderer().(*externalLinkRenderer)
	if !ok {
		t.Fatal("NewExternalLinkRenderer must return *externalLinkRenderer")
	}
	return r
}

func newTestLink(destination, title string) *ast.Link {
	l := &ast.Link{}
	l.Destination = []byte(destination)
	if title != "" {
		l.Title = []byte(title)
	}
	return l
}

func newTestAutoLink(t ast.AutoLinkType, value string) *ast.AutoLink {
	return ast.NewAutoLink(t, ast.NewTextSegment(text.NewSegment(0, len(value))))
}

// TestExternalLinkRendererErrorPaths sweeps a failing writer through the
// renderer entry points so every `return ..., err` branch is exercised.
// These branches cannot fail through the markdown pipeline (the underlying
// buffer never errors), so they are only reachable with an injected writer.
func TestExternalLinkRendererErrorPaths(t *testing.T) {
	r := newTestExternalLinkRenderer(t)

	titledLink := newTestLink(testExtURL, "Example")
	attributedLink := newTestLink(testExtURL, "")
	attributedLink.SetAttribute([]byte(testAttrName), []byte(testAttrVal))

	attributedAutoLink := newTestAutoLink(ast.AutoLinkURL, testExtURL)
	attributedAutoLink.SetAttribute([]byte(testAttrName), []byte(testAttrVal))

	emailSrc := []byte(testEmail)
	emailAutoLink := newTestAutoLink(ast.AutoLinkEmail, testEmail)
	urlSrc := []byte(testExtURL)
	urlAutoLink := newTestAutoLink(ast.AutoLinkURL, testExtURL)

	for budget := 0; budget < sweepBudget; budget++ {
		// Fresh writer per call so each entry point sees the full budget sweep.
		_, _ = r.renderLink(&failingWriter{budget: budget}, nil, ast.NewParagraph(), true)
		_, _ = r.renderAutoLink(&failingWriter{budget: budget}, urlSrc, ast.NewParagraph(), true)

		// renderLinkEnter: href write, title write, attribute render, '>' write.
		_, _ = r.renderLinkEnter(&failingWriter{budget: budget}, titledLink)
		_, _ = r.renderLinkEnter(&failingWriter{budget: budget}, attributedLink)

		// renderLinkExit: indicator write before </a>.
		_, _ = r.renderLinkExit(&failingWriter{budget: budget}, newTestLink(testExtURL, ""))

		// renderAutoLink: scheme write, href close, label, indicator, </a>.
		_, _ = r.renderAutoLink(&failingWriter{budget: budget}, urlSrc, urlAutoLink, true)
		_, _ = r.renderAutoLink(&failingWriter{budget: budget}, emailSrc, emailAutoLink, true)

		// writeAutoLinkHrefClose: attribute-bearing path (quote + attributes + '>').
		_ = writeAutoLinkHrefClose(&failingWriter{budget: budget}, attributedAutoLink)
	}
}

// TestExternalLinkRendererHappyPath pins the rendered output for the paths
// the error sweep above deliberately breaks mid-write.
func TestExternalLinkRendererHappyPath(t *testing.T) {
	r := newTestExternalLinkRenderer(t)

	titled := newTestLink(testExtURL, "Example")
	out := &bytes.Buffer{}
	buf := bufio.NewWriter(out)
	if _, err := r.renderLinkEnter(buf, titled); err != nil {
		t.Fatalf("renderLinkEnter: %v", err)
	}
	if _, err := r.renderLinkExit(buf, titled); err != nil {
		t.Fatalf("renderLinkExit: %v", err)
	}
	if err := buf.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	got := out.String()
	want := `<a href="` + testExtURL + `" title="Example">` + externalLinkIndicator + `</a>`
	if got != want {
		t.Fatalf("rendered anchor = %q, want %q", got, want)
	}

	autoOut := &bytes.Buffer{}
	autoBuf := bufio.NewWriter(autoOut)
	if _, err := r.renderAutoLink(autoBuf, []byte(testExtURL), newTestAutoLink(ast.AutoLinkURL, testExtURL), true); err != nil {
		t.Fatalf("renderAutoLink: %v", err)
	}
	if err := autoBuf.Flush(); err != nil {
		t.Fatalf("flush autolink: %v", err)
	}
	autoGot := autoOut.String()
	autoWant := `<a href="` + testExtURL + `">` + testExtURL + externalLinkIndicator + `</a>`
	if autoGot != autoWant {
		t.Fatalf("rendered autolink = %q, want %q", autoGot, autoWant)
	}
}
