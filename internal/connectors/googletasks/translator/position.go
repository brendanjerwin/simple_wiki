package translator

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// positionParseBase is the numeric base (decimal) used when parsing
// Google Tasks position strings.
const positionParseBase = 10

// positionParseBitSize is the bit-size target (int64) passed to
// strconv.ParseInt when parsing position strings.
const positionParseBitSize = 64

// positionDigits is the number of digits Google Tasks uses for the
// `position` field on tasks returned from the API. Empirically, Tasks
// returns 20-digit zero-padded decimal strings (e.g.,
// "00000000000000000001"). This isn't documented but has been stable
// since the API launched. The lex order on the strings encodes the
// task order.
const positionDigits = 20

// PositionToSortOrder converts a Google Tasks `position` lex string to
// a wiki sparse int64 sort_order.
//
// Algorithm: Tasks positions are zero-padded decimal strings of fixed
// width (typically 20 digits). They sort lexicographically equivalent
// to numerically. We strip leading zeros and parse with the standard
// int64 base-10 parser. If the resulting numeric value would overflow
// int64 (>= 2^63), we truncate to math.MaxInt64 — Tasks's positions
// are issued by Google's allocator and in practice fit comfortably in
// 18 digits, but the safety net keeps the function total.
//
// Empty positions return 0.
//
// Round-trip note: PositionToSortOrder ↔ SortOrderToPosition is
// **not** byte-identical for arbitrary inputs. Tasks issues positions
// from a server-side sequence we don't control, and the wiki uses
// sparse int64 multiples of 1000 by convention. The round trip
// preserves *order* (positions a < b imply sort_orders a' < b' and
// vice versa), which is the only invariant the wiki's UI cares about.
// Documented as a known asymmetry; the per-checklist Subscription's
// item_id_map is the source of identity, not position equality.
func PositionToSortOrder(position string) int64 {
	if position == "" {
		return 0
	}
	// Strip leading zeros to bring the value into the range int64
	// can represent. ParseInt rejects strings with leading + or
	// leading zeros only when they overflow; in practice Tasks
	// positions like "00000000000000000001" parse cleanly with no
	// stripping. We do an explicit TrimLeft here so a 20-digit
	// position whose top bit would set above int64's max also has
	// a chance to pass.
	trimmed := strings.TrimLeft(position, "0")
	if trimmed == "" {
		// All zeros — first task.
		return 0
	}
	n, err := strconv.ParseInt(trimmed, positionParseBase, positionParseBitSize)
	if err != nil {
		// Overflow or non-decimal characters: clamp to int64 max
		// (preserves "this is later than anything we've seen"
		// ordering) rather than corrupting the wiki ordering with
		// a silent zero.
		return math.MaxInt64
	}
	return n
}

// SortOrderToPosition converts a wiki int64 sort_order to a Google
// Tasks-style fixed-width zero-padded decimal `position` string.
//
// Negative sort_orders are clamped to 0 — Tasks doesn't model negative
// positions, and the wiki's convention is sparse positive multiples of
// 1000. Negative orders here would indicate caller error; we prefer to
// clamp + render a valid string rather than return an error from a
// function the caller treats as total.
//
// The result is `positionDigits` characters wide, zero-padded on the
// left. Note: Tasks's `tasks.insert` accepts a `previous` parameter for
// relative positioning and ignores any `position` field on the
// request body, so this function is primarily used for diagnostic /
// fingerprinting purposes (see Phase 5 sync notes in plan §3,
// "Outbound idempotence").
func SortOrderToPosition(sortOrder int64) string {
	if sortOrder < 0 {
		sortOrder = 0
	}
	return fmt.Sprintf("%0*d", positionDigits, sortOrder)
}
