# Code and Structure Conventions

<!--toc:start-->

- [General](#general)
- [Development Environment](#development-environment)
- [Frontend JavaScript](#frontend-javascript)
- [Testing](#testing)
- [Fixing Problems](#fixing-problems)
<!--toc:end-->

## General

- Make Uncle Bob proud.
- prefer modern go. Update idioms to modern approaches as you see them. Boyscout rule.
- Prefer `any` over `interface{}` for clarity and to align with modern Go standards.
- Prefer standard Go idioms and approaches, such as using `go:generate` for code generation tasks.
- Generated files **should be committed** to the repository. This ensures that developers can build and test the project without needing to have all code generation tools installed locally. Any files created or modified by `go generate ./...` should be included in commits.
- prefer IoC approaches. Make \*-er interfaces for all the things!
- take a defensive coding approach. Check inputs, assert preconditions and invariants. Assert assumptions.
- For preconditions and invariants, prefer returning `error` over `panic`. Panics should be reserved for truly exceptional situations. For conditions that indicate a programming or configuration error, such as a missing dependency, functions should return an error. This allows the caller to handle the problem more gracefully.

- When adding a new component to the system, ensure it is also added to the C4 model in `docs/workspace.dsl`. This keeps our architectural documentation up-to-date. Each component should include a `properties` block specifying the source file.

  **Example:**

  ```dsl
  myNewComponent = component "My New Component" "Does something awesome." "Go" {
      properties {
          file "internal/path/to/my_new_component.go"
      }
  }
  ```

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

## Development Environment

- Use [Devbox](https://www.jetpack.io/devbox/) by Jetify to create isolated, reproducible development environments.
- Add new dependencies via `devbox add <package>`. This ensures the `devbox.json` and `devbox.lock` files are updated correctly.

## Frontend JavaScript

- All JavaScript frontend code should be located under the `static/js` directory.
- Organize frontend JavaScript files within appropriate subdirectories under `static/js` (e.g., `web-components`, `utils`).

## TDD

- Be Test-Driven. Write the test first, then write the code to make the tests pass.
- When adding a function or method, follow a strict TDD workflow:

  1. First, add the function signature with a no-op implementation (a "skeleton").
  2. Next, write a failing test that defines the desired behavior.
  3. Finally, write the implementation code to make the test pass, then refactor.

  This ensures that code is written to meet specific, testable requirements from the start.

  **Example:** Adding a `Sum` function.

  **Step 1: Create the skeleton function**

  ```go
  // Sum adds two integers.
  func Sum(a, b int) int {
      return 0 // No-op implementation
  }
  ```

  **Step 2: Write a failing test**

  ```go
  // in sum_test.go
  Describe("Sum", func() {
      It("should add two numbers", func() {
          Expect(Sum(2, 3)).To(Equal(5))
      })
  })
  ```

  Running the test at this point will fail, as expected.

  **Step 3: Implement the function to pass the test**

  ```go
  // Sum adds two integers.
  func Sum(a, b int) int {
      return a + b
  }
  ```

## Testing

- Prefer Gomego/Ginkgo for testing. Context-Specification style. Nest `describe` blocks to build up context. Don't bother with `context` blocks.
- Don't do actions in the `It` blocks. The `It` blocks should only contain assertions. All setup (**Arrange**) and execution (**Act**) should be done in `BeforeEach` blocks within the `Describe` or `When` blocks. This allows for reusing context to add additional assertions later.

  **Bad:** Action inside the `It` block.

  ```go
  Describe("a component", func() {
    When("in a certain state", func() {
      It("should do a thing", func() {
        // Arrange
        component := setupComponent()

        // Act
        result, err := component.DoSomething()

        // Assert
        Expect(err).NotTo(HaveOccurred())
        Expect(result).To(Equal("expected result"))
      })
    })
  })
  ```

  **Good:** Action moved to `BeforeEach`.

  ```go
  Describe("a component", func() {
    When("in a certain state", func() {
      var (
        component *Component
        result    string
        err       error
      )

      BeforeEach(func() {
        // Arrange
        component = setupComponent()

        // Act
        result, err = component.DoSomething()
      })

      It("should not return an error", func() {
        // Assert
        Expect(err).NotTo(HaveOccurred())
      })

      It("should return the correct result", func() {
        // Assert
        Expect(result).To(Equal("expected result"))
      })
    })
  })
  ```

- When asserting an error, check for the specific error type or message. Do not just check that an error is not `nil`. This ensures that the test is validating the specific error that is expected to be returned.

  **Bad:**

  ```go
  Expect(err).To(HaveOccurred())
  ```

  **Good:**

  ```go
  Expect(err).To(MatchError("specific error message"))
  ```

- Use the `Describe` blocks first to describe the function/component being tested, then use nested `When` blocks to establish the scenarios. Besides the basic `It(text: "Should exist"` tests, everything should be in those nested "When" blocks.
- Include a blank line between all the various Ginkgo blocks. This makes it easier to read the tests.

## Fixing Problems

- Do not obfuscate errors. When a function returns an error, inspect it to return an appropriate error to the caller. Do not wrap it in a generic error that hides the original cause or assumes a specific failure mode that may not be true. For example, if a read operation fails, don't automatically assume the file was "not found" if the underlying error could be something else, like a permissions issue.

  **Bad:**

  ```go
  // This is bad because it assumes any error from ReadFrontMatter means "not found".
  _, _, err := s.PageReadWriter.ReadFrontMatter(req.Page)
  if err != nil {
      return nil, status.Errorf(codes.NotFound, "page not found: %s", req.Page)
  }
  ```

  **Good:**

  ```go
  // This is better. We check for a specific error type and handle it,
  // falling back to a more general error for unexpected cases.
  _, _, err := s.PageReadWriter.ReadFrontMatter(req.Page)
  if err != nil {
      if os.IsNotExist(err) {
          return nil, status.Errorf(codes.NotFound, "page not found: %s", req.Page)
      }
      return nil, status.Errorf(codes.Internal, "failed to read frontmatter: %v", err)
  }
  ```

- If a problem is due to an invalid parameter, don't just fix the parameter value. _also_ add an input validation to the function/method receiving the parameter such that the error being fixed is perfectly clear to the next developer.
- Do not use `recover` to hide panics. A panic indicates a serious bug that should crash the program during development and be fixed. Catching panics in handlers can obfuscate the problem and make debugging difficult. Instead, write defensive code to prevent panics in the first place, for example by checking for `nil`.

## README

- When updating the readme, match the tone of voice in the rest of the README. Its the face of the project. Marketing matters.
