# Issue #1349 — Claude-style chat composer

Reference: https://github.com/brendanjerwin/simple_wiki/issues/1349
Design decided from issue comments + Anthropic help-center research (see issue comments).

## Decided design

1. **Send button visibility swap.** The send control is an icon-only round
   button (`fa-solid fa-arrow-up`) that is completely absent (not merely
   disabled) when the composer is empty. It appears — swapping into the same
   bottom-right slot — whenever the draft has text (non-empty after trim) or
   pending attachments. Draft emptiness is tracked via a private `@state()
   _hasDraftText` fed by an `@input` handler on the textarea (the textarea is
   uncontrolled; it is not a Lit reactive prop). `sendMessage()` keeps its
   early-return when there is nothing to send; Enter-to-send /
   Shift+Enter-newline are unchanged.
2. **Subtle "+" attach button.** The attach control loses its boxed border
   styling and renders as a bare `fa-plus` icon at the bottom-left of the same
   rounded composer container as the textarea. The existing attach menu
   (Upload file / Photo library / Camera) opens from it; menu content is
   unchanged.
3. **Composer container.** The `.input-area` contents are restructured into a
   single rounded `.composer` container holding the attachment-chips row and
   the input row (plus button, textarea, send slot). The drag-over highlight
   moves to the composer wrapper.
4. **Chips stay inline and small.** Attachment/pasted images render inside the
   composer content area above the textarea, exactly as today, keeping the
   current small 20px thumbnails — we explicitly do NOT adopt Claude's larger
   thumbnails.
5. **Existing stop semantics preserved.** The Stop button stays in the
   thinking indicator for the whole active turn; the composer keeps its
   `?disabled` gating when chat is unavailable (`inputEnabled`).

## Work order

- Worker A (child worktree `issue-1349-composer`, branch off
  `brendanjerwin/chat-entry-box-claude-style-plus-send-button-beh`):
  TDD implementation of 1–5 in `static/js/web-components/page-chat-panel.ts`
  and `page-chat-panel.test.ts` (+ `e2e/tests/page-chat-panel.spec.ts` if
  needed). Failing tests first for: send hidden when empty; send shown with
  text only; send shown with attachments only (no text); chips render inside
  the composer; plus button has no boxed border. Run
  `devbox run fe:test -- static/js/web-components/page-chat-panel.test.ts`,
  `devbox run fe:lint`, `devbox run fe:typecheck`,
  `devbox run e2e:test -- e2e/tests/page-chat-panel.spec.ts`. Commit on the
  child branch; report SHA.
- Worker B (validation): scoped checks on the same branch + Storybook-based
  composer screenshots (empty state; with text; with image attachment) saved
  to `docs/screenshots/page-chat-panel/` for PR embedding.
- Worker C (review): plan-vs-code review of final branch diff against this
  plan.
- Coordinator: push branch, open PR referencing #1349 with embedded raw
  GitHub screenshot URLs (per managed skill `component-pr-screenshots`), then
  verify CI gates.

## Help docs

`internal/syspage/embedded/help_chat.md` "Attaching files and images" section
must be updated in the same PR if any user-visible wording changes (chips stay
inline; send-button-when-content wording may need a sentence).
