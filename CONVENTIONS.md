# Code and Structure Conventions

<!--toc:start-->

- [General](#general)
- [Testing](#testing)
- [Fixing Problems](#fixing-problems)
<!--toc:end-->

## General

- Make Uncle Bob proud.
- prefer modern go. Update idioms to modern approaches as you see them. Boyscout rule.
- prefer IoC approaches. Make \*-er interfaces for all the things!
- take a defensive coding approach. Check inputs, assert preconditions and invariants. Assert assumptions.
- For preconditions and invariants, prefer returning `error` over `panic`. Panics should be reserved for truly exceptional situations. For conditions that indicate a programming or configuration error, such as a missing dependency, functions should return an error. This allows the caller to handle the problem more gracefully.

  When checking for a dependency within an HTTP handler, you should check for the dependency and return an appropriate HTTP error if it's missing.

  Example:

  ```go
  func (s *Site) handlePrintLabel(c *gin.Context) {
    if s.FrontmatterIndexQueryer == nil {
      c.JSON(http.StatusInternalServerError, gin.H{"error": "Frontmatter index is not available"})
      return
    }

    //...
  }
  ```

## TDD

- Be Test-Driven. Write the test first, then write the code to make the tests pass.

## Testing

- Prefer Gomego/Ginkgo for testing. Context-Specification style. Nest `describe` blocks to build up context. Don't bother with `context` blocks.
- Don't do actions in the `It` blocks. Do them as `beforeEach` blocks in the `Describe` blocks. We want to reuse context to add additional assertions later.
- Use the `Describe` blocks first to describe the function/component being tested, then use nested `When` blocks to establish the scenarios. Besides the basic `It(text: "Should exist"` tests, everything should be in those nested "When" blocks.
- Include a blank line between all the various Ginkgo blocks. This makes it easier to read the tests.

## Fixing Problems

- If a problem is due to an invalid parameter, don't just fix the parameter value. _also_ add an input validation to the function/method receiving the parameter such that the error being fixed is perfectly clear to the next developer.
