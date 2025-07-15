### Test File Conformance Checklist

To ensure consistency and readability, all test files should adhere to the following guidelines:

-   **Context-Specification Style:**
    -   `describe` blocks are nested to build up context.
    -   `context` blocks are avoided.
    -   `describe` blocks are used first to describe the function/component being tested.
    -   Nested `When` blocks are used to establish specific scenarios. For frameworks without native `When` blocks (e.g., Mocha), `describe` blocks can be used to serve this purpose.
    -   Basic `It("should exist")` tests (or equivalent) are placed directly under the main `describe` block.
    -   All other `It` blocks are placed within `When` blocks.
    -   **Important**: Use "when" in `describe` block descriptions to establish scenarios, not in `it` block descriptions. The `it` blocks should describe the expected behavior or outcome.

-   **No Actions in `It` Blocks:**
    -   `It` blocks contain only assertions.
    -   All setup (Arrange) and execution (Act) are performed in `BeforeEach` (or equivalent, e.g., `beforeEach` in JavaScript) blocks within the `Describe` or `When` blocks.

-   **Specific Error Assertions:**
    -   When asserting errors, check for the specific error type or message, not just for the presence of an error.

-   **Consistent Formatting:**
    -   Include a blank line between all `Describe`, `When`, `BeforeEach`, and `It` blocks to improve readability.