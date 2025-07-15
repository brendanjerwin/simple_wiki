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
- Prefer modern, idiomatic approaches for the language in use. Update idioms as you see them to align with current best practices (Boyscout Rule).
- **Type Clarity**: Prioritize clear and explicit type declarations. For Go, prefer `any` over `interface{}`.
- **Standard Idioms and Tooling**: Prefer standard idioms and approaches for the language in use. Leverage appropriate code generation tools when beneficial (e.g., `go:generate` for Go). For JavaScript, utilize tools like `bun build` for bundling and `web-test-runner` for testing.
- Generated files **should be committed** to the repository. This ensures that developers can build and test the project without needing to have all code generation tools installed locally. Any files created or modified by `go generate ./...` should be included in commits.
- prefer IoC approaches. Make \*-er interfaces for all the things!
    - **Defensive Error Handling**: Take a defensive coding approach. Check inputs, assert preconditions, and enforce invariants. For recoverable issues, prefer returning an error or throwing an
  exception rather than causing a program crash (panic). Crashes/panics should be reserved for truly exceptional and unrecoverable situations. This allows the caller to handle the problem more
  gracefully.

    **Go Example (Returning Error)**:
    ```go
    func MyFunction(input int) error {
        if input < 0 {
            return fmt.Errorf("input cannot be negative")
        }
        // ...
        return nil
    }
    ```

    **JavaScript Example (Throwing Exception)**:
    ```javascript
    function myFunction(input) {
      if (input < 0) {
          throw new Error('Input cannot be negative');
      }
      // ...
      return true;
  }
  ```

## Development Environment

- Use [Devbox](https://www.jetpack.io/devbox/) by Jetify to create isolated, reproducible development environments.
- Add new dependencies via `devbox add <package>`. This ensures the `devbox.json` and `devbox.lock` files are updated correctly.

## Frontend JavaScript

- All JavaScript frontend code and project files should be located under the `static/js` directory.
- This includes JavaScript source files, test files, configuration files (e.g., `vitest.config.js`), and package management files (e.g., `package.json`, `bun.lock`).
- Use Bun as the package manager for modern JavaScript tooling and faster performance.
- Since this is primarily a Go project, JavaScript is only for the frontend portion and should be contained within the `static/js` directory.
- Organize frontend JavaScript files within appropriate subdirectories under `static/js` (e.g., `web-components`, `utils`).
- Test files should be placed next to the production code they test, using the `.test.js` suffix. For example, `wiki-search.js` should have its tests in `wiki-search.test.js` in the same directory.

## TDD

- Be Test-Driven. Write the test first, then write the code to make the tests pass.
- When adding a new capability (function, method, component), follow a strict TDD workflow: This process is language-agnostic, and the Go examples below illustrate the key steps.

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

   108→## Testing
   109→
   110→This section outlines best practices for testing, applicable across different programming languages used in the project. The core principles emphasize test-driven development (TDD), clear test structure, and meaningful assertions.
   111→
   112→- Prefer Context-Specification style for testing. Nest `describe` blocks to build up context. Don't bother with `context` blocks.
   113→- Don't do actions in the `It` blocks. The `It` blocks should only contain assertions. All setup (**Arrange**) and execution (**Act**) should be done in `BeforeEach` blocks (or equivalent, depending on the testing framework) within the `Describe` or `When` blocks. This allows for reusing context to add additional assertions later.
   114→
   115→  **Bad:** Action inside the `It` block (Go example).
   116→
   117→  ```go
   118→  Describe("a component", func() {
   119→    When("in a certain state", func() {
   120→      It("should do a thing", func() {
   121→        // Arrange
   122→        component := setupComponent()
   123→
   124→        // Act
   125→        result, err := component.DoSomething()
   126→
   127→        // Assert
   128→        Expect(err).NotTo(HaveOccurred())
   129→        Expect(result).To(Equal("expected result"))
   130→      })
   131→    })
   132→  })
   133→  ```
   134→
   135→  **Good:** Action moved to `BeforeEach` (Go example).
   136→
   137→  ```go
   138→  Describe("a component", func() {
   139→    When("in a certain state", func() {
   140→      var (
   141→        component *Component
   142→        result    string
   143→        err       error
   144→      )
   145→
   146→      BeforeEach(func() {
   147→        // Arrange
   148→        component = setupComponent()
   149→
   150→        // Act
   151→        result, err = component.DoSomething()
   152→      })
   153→
   154→      It("should not return an error", func() {
   155→        // Assert
   156→        Expect(err).NotTo(HaveOccurred())
   157→      })
   158→
   159→      It("should return the correct result", func() {
   160→        // Assert
   161→        Expect(result).To(Equal("expected result"))
   162→      })
   163→    })
   164→  })
   165→  ```
   166→
   167→  **Good:** Action moved to `beforeEach` (JavaScript Example `static/js/web-components/wiki-search.test.js`).
   168→
   169→  ```javascript
   170→  describe('when component is connected to DOM', () => {
   171→    let addEventListenerSpy;
   172→    
   173→    beforeEach(async () => {
   174→      addEventListenerSpy = sinon.spy(window, 'addEventListener');
   175→      // Re-create the element to trigger connectedCallback
   176→      el = await fixture(html`<wiki-search></wiki-search>`);
   177→      await el.updateComplete;
   178→    });
   179→    
   180→    it('should add keydown event listener', () => {
   181→      expect(addEventListenerSpy).to.have.been.calledWith('keydown', el._handleKeydown);
   182→    });
   183→  });
   184→  ```
   185→
   186→- When asserting an error, check for the specific error type or message. Do not just check that an error is not `nil`. This ensures that the test is validating the specific error that is expected to be returned.
   187→
   188→  **Bad:** (Go example)
   189→
   190→  ```go
   191→  Expect(err).To(HaveOccurred())
   192→  ```
   193→
   194→  **Good:** (Go example)
   195→
   196→  ```go
   197→  Expect(err).To(MatchError("specific error message"))
   198→  ```
   199→
   200→- Use the `Describe` blocks first to describe the function/component being tested, then use nested `When` blocks to establish the scenarios. Besides the basic `It(text: "Should exist"` tests, everything should be in those nested "When" blocks.
   201→- Include a blank line between all the various Ginkgo blocks. This makes it easier to read the tests.

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
