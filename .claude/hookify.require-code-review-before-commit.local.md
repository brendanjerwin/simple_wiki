---
name: require-code-review-before-commit
enabled: true
event: bash
action: block
conditions:
  - field: command
    operator: regex_match
    pattern: git\s+(commit|push)
  - field: command
    operator: not_contains
    pattern: "[REVIEWED:"
---

**BLOCKED: Critical Code Review Required Before Commit/Push**

This commit/push is **blocked** because it doesn't include a review summary marker.

**To proceed, you MUST:**

1. **Run the expert panel code review** using a general-purpose sub-agent with this prompt:

   "You are a panel of distinguished experts conducting a critical code review:

   - **Uncle Bob (Robert C. Martin)**: Clean Code principles, SOLID, naming conventions
   - **Kent Beck**: Test-driven development, refactoring, simplicity
   - **Linus Torvalds**: Systems programming, efficiency, no-nonsense critique
   - **Rich Hickey**: Simplicity vs complexity, state management, design

   **IMPORTANT**: Also review against the project's CONVENTIONS.md / CLAUDE.md guidelines.

   Review all modified files in the current git status. For each issue found:
   - Severity: Critical / High / Medium / Low
   - Expert: Which expert identified it
   - Issue: Clear description with enough context for the user to understand
   - Code snippet: Show the relevant code
   - Fix: Recommended solution

   Be thorough but practical. Small fixes should be actionable immediately.
   Ensure all code follows the established conventions in CLAUDE.md."

2. **Fix issues discovered:**
   - **Small fixes**: Apply them immediately without asking
   - **Medium/Large issues**: Present to user with AskUserQuestion:
     - Include enough context for the user to judge the issue
     - Show the problematic code snippet
     - Explain why it's an issue
     - Always include an "Explain this issue more" option for when user needs more context
     - Let user decide: Fix it, Skip it, or Explain more

3. **Include review marker in commit message:**
   After completing the review, add a review summary marker to your commit message:

   ```
   [REVIEWED: X issues found, Y fixed, Z skipped (reason)]
   ```

   Examples:
   - `[REVIEWED: 3 issues found, 3 fixed, 0 skipped]`
   - `[REVIEWED: 2 issues found, 1 fixed, 1 skipped (style preference)]`
   - `[REVIEWED: 0 issues found]`

   The marker MUST be included in the commit message (via -m flag or HEREDOC) for the commit to proceed.

**This block cannot be bypassed without completing the review and including the marker.**
