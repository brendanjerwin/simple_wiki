// Test fixtures for the error-mishandling rules in ../rules.yml.
// Run with: scripts/run_opengrep_test.sh
//
// Lines marked "RULEID" annotate matches for the named rule;
// "OK" lines should not match. This file intentionally contains
// anti-pattern code; nothing here is imported or compiled by the application.
//
//revive:disable:add-constant,unused-receiver
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

// ----- connector-service-no-request-kind-switch ---------------------------

type connectorRequest struct{}

func (*connectorRequest) GetConnectorKind() int { return 0 }

func badDirectConnectorKindSwitch(req *connectorRequest) int {
	// ruleid: go.connector-service-no-request-kind-switch
	switch req.GetConnectorKind() {
	case 1:
		return 1
	default:
		return 0
	}
}

func badLocalConnectorKindSwitch(req *connectorRequest) int {
	// ruleid: go.connector-service-no-request-kind-switch
	kind := req.GetConnectorKind()
	switch kind {
	case 1:
		return 1
	default:
		return 0
	}
}

func goodConnectorKindDispatch(kind int) int {
	// ok: go.connector-service-no-request-kind-switch
	switch kind {
	case 1:
		return 1
	default:
		return 0
	}
}
