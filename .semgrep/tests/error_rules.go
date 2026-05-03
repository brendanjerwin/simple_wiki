// Test fixtures for the error-mishandling rules in ../rules.yml.
// Run with: scripts/run_opengrep_test.sh
//
// Lines marked "RULEID" annotate matches for the named rule;
// "OK" lines should not match. This file intentionally contains
// anti-pattern code; nothing here is imported or compiled by the application.
package tests

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

// ----- error-wrapped-with-percent-v ---------------------------------------

func badWrap(err error) error {
	// ruleid: go.error-wrapped-with-percent-v
	return fmt.Errorf("oops: %v", err)
}

func goodWrap(err error) error {
	// ok: go.error-wrapped-with-percent-v
	return fmt.Errorf("oops: %w", err)
}

// ----- error-discarded-with-blank-identifier ------------------------------

func badDiscard() {
	// ruleid: go.error-discarded-with-blank-identifier
	_ = stepThatCanFail()
}

func goodDiscard(resp *http.Response) {
	// ok: go.error-discarded-with-blank-identifier
	_ = resp.Body.Close()
}

func stepThatCanFail() error { return nil }

// ----- handler-returns-nil-after-error-logged -----------------------------

func badHandler() error {
	// ruleid: go.handler-returns-nil-after-error-logged
	if err := stepThatCanFail(); err != nil {
		slog.Error("oh no", "err", err)
		return nil
	}
	return nil
}

func goodHandler() error {
	if err := stepThatCanFail(); err != nil {
		// ok: go.handler-returns-nil-after-error-logged
		return fmt.Errorf("step: %w", err)
	}
	return nil
}

// ----- errors-As-without-target-pointer -----------------------------------

type myErr struct{}

func (e *myErr) Error() string { return "" }

func badAs(err error) bool {
	var target *myErr
	// ruleid: go.errors-As-without-target-pointer
	return errors.As(err, target)
}

func goodAs(err error) bool {
	var target *myErr
	// ok: go.errors-As-without-target-pointer
	return errors.As(err, &target)
}

// ----- error-classified-by-status-code-only -------------------------------

var ErrUnauthorized = errors.New("unauthorized")

func badClassifySingleReturn(resp *http.Response) error {
	// ruleid: go.error-classified-by-status-code-only
	if resp.StatusCode == 401 {
		return ErrUnauthorized
	}
	return nil
}

func badClassifyMultiReturn(resp *http.Response) (int, error) {
	// ruleid: go.error-classified-by-status-code-only
	if resp.StatusCode >= 500 {
		return 0, ErrUnauthorized
	}
	return 0, nil
}

func goodClassify(resp *http.Response, body []byte) error {
	// ok: go.error-classified-by-status-code-only
	if resp.StatusCode == 401 {
		return classifyByBody(body)
	}
	return nil
}

func classifyByBody(_ []byte) error { return nil }
