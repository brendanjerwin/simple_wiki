# Code and Structure Conventions

<!--toc:start-->

- [General](#general)
- [Development Environment](#development-environment)
- [Frontend JavaScript](#frontend-javascript)
- [Storybook](#storybook)
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
      throw new Error("Input cannot be negative");
    }
    // ...
    return true;
  }
  ```

## Development Environment

- Use [Devbox](https://www.jetpack.io/devbox/) by Jetify to create isolated, reproducible development environments.
- Add new dependencies via `devbox add <package>`. This ensures the `devbox.json` and `devbox.lock` files are updated correctly.
- Use either `devbox shell` (for an interactive shell) or `devbox run <command>` (for running specific commands) to work within the Devbox environment.
- When possible, use scripts defined in `devbox.json` to run commands. Run `devbox run` to see a list of available scripts.

## Frontend JavaScript

- All JavaScript frontend code and project files should be located under the `static/js` directory.
- This includes JavaScript source files, test files, configuration files (e.g., `vitest.config.js`), and package management files (e.g., `package.json`, `bun.lock`).
- Use Bun as the package manager for modern JavaScript tooling and faster performance.
- Since this is primarily a Go project, JavaScript is only for the frontend portion and should be contained within the `static/js` directory.
- Organize frontend JavaScript files within appropriate subdirectories under `static/js` (e.g., `web-components`, `utils`).
- Test files should be placed next to the production code they test, using the `.test.js` suffix. For example, `wiki-search.js` should have its tests in `wiki-search.test.js` in the same directory.
- **Scalar Variable Units**: Always include units in scalar variable names when the unit is meaningful. For example, use `timeoutMs` instead of `timeout`, `delaySeconds` instead of `delay`, `heightPx` instead of `height`. This makes the code self-documenting and prevents unit confusion.

## Storybook

Storybook is used for developing and documenting UI components in isolation. Follow these principles when creating and maintaining Storybook stories:

### Story Creation Rules

- **ALWAYS use actual components**: Stories must render the actual component, never create mock HTML structures that simulate the component's appearance.

  **Bad:** Creating mock HTML divs to simulate a dialog component:

  ```typescript
  render: (args) => html`
    <div style="position: fixed; inset: 0; background: rgba(0,0,0,0.5);">
      <div style="background: white; border-radius: 8px;">
        <h2>Fake Dialog Title</h2>
        <p>Mock content that simulates the real component</p>
      </div>
    </div>
  `;
  ```

  **Good:** Using the actual component and setting its properties:

  ```typescript
  render: (args) => html`
    <my-dialog-component .open="${args.open}" .loading="${true}">
    </my-dialog-component>
  `;
  ```

- **Mock data, not components**: It's perfectly acceptable (and encouraged) to use mock data to populate components with realistic content. The distinction is:

  - ✅ Mock data: Providing sample arrays, objects, or strings as component props
  - ❌ Mock components: Creating fake HTML elements instead of using the real component

- **Leverage component state**: Components often have internal state properties that control loading, error, and other UI states. Set these properties directly in stories to demonstrate different visual states without requiring backend connections.

- **Test all component states**: Create stories that demonstrate all major visual states of a component (loading, error, success, empty, etc.) by manipulating the component's properties.

### Story Organization

- Group related stories under a logical hierarchy using the `title` metadata
- Use descriptive story names that clearly indicate what state or variation is being demonstrated
- Include meaningful controls via `argTypes` for properties that users should be able to manipulate interactively

### Development Integration

- Storybook runs as part of the `devbox services` workflow for seamless development
- Use `devbox run storybook` to run Storybook standalone
- Use `devbox run storybook:build` to build static Storybook files
- Stories are automatically deployed to Chromatic for visual regression testing

### Actions and Interactions

Every story should include comprehensive event logging and interaction testing for user behaviors:

#### Custom Action Logging Setup

Since we use Storybook's base configuration, we implement custom action logging:

```typescript
// Custom action logger for Storybook
const action = (name: string) => (event: Event) => {
  console.log(`🎬 Action: ${name}`, {
    type: event.type,
    target: event.target,
    detail: (event as CustomEvent).detail,
    timestamp: new Date().toISOString(),
  });
};
```

#### Event Binding in Stories

- **Bind action loggers to all relevant events** in your story render functions:

  ```typescript
  render: (args) => html`
    <my-component
      @click=${action('component-clicked')}
      @input=${action('input-changed')}
      @custom-event=${action('custom-event')}>
    </my-component>
  `,
  ```

#### Interactive Testing Stories

- **Create dedicated interactive testing stories** for complex components that demonstrate:

  - User workflows (form filling, multi-step interactions)
  - Keyboard shortcuts and accessibility features
  - Error states and recovery scenarios
  - Real-world usage patterns

- **Use descriptive story names** that indicate the interaction being tested:
  - `InteractiveFormTesting` - for comprehensive form interaction testing
  - `KeyboardShortcuts` - for keyboard navigation and shortcuts
  - `DropdownInteractions` - for dropdown open/close/selection behaviors
  - `ErrorRecovery` - for testing error states and user recovery paths

#### Documentation and Context

- **Add comprehensive story descriptions** using `parameters.docs.description.story`
- **Provide clear instructions** for what users should test and what events to watch for in the browser console
- **Include visual context** with appropriate styling and layout to simulate real usage
- **Always mention "Open the browser developer tools console to see the action logs"** in interactive story descriptions

#### Event Payload Testing

- **Verify event payloads** contain correct data by checking the browser console logs
- **Test data flow** by creating stories that demonstrate how component state changes affect event payloads
- **Document expected event structure** in story descriptions

#### Example Pattern

```typescript
export const InteractiveExample: Story = {
  render: (args) => html`
    <div style="padding: 20px; background: #f0f8ff;">
      <h3>Component Interaction Test</h3>
      <p>Instructions for testing...</p>
      <my-component
        @event1=${action("event1-triggered")}
        @event2=${action("event2-with-data")}
      >
      </my-component>
      <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
        Watch the browser console (F12) to see triggered events logged.
      </p>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story:
          "Detailed description of what to test and expected behavior. Open the browser developer tools console to see the action logs.",
      },
    },
  },
};
```

### Storybook Focus

Storybook is used for **visual component exploration and manual interaction testing**. Functional testing should be handled by the regular unit test suite.

- **Visual documentation**: Use Storybook to showcase component variants and visual states
- **Manual interaction testing**: Use action logging to track user interactions and events
- **Component exploration**: Provide interactive controls for real-time property manipulation
- **Design system**: Maintain a centralized view of all UI components for consistency

Storybook should focus on visual and interactive aspects, while automated functional testing is handled by the project's comprehensive unit test suite.

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

## Testing

This section outlines best practices for testing, applicable across different programming languages used in the project. The core principles emphasize test-driven development (TDD), clear test structure, and meaningful assertions.

### Running the Application

The application can be run locally using `devbox services`, which manages the development environment and builds the frontend code automatically.

- **Start the Application**:

  ```bash
  devbox services start
  ```

- **Stop the Application**:

  ```bash
  devbox services stop
  ```

The `devbox services` command uses the configuration defined in `process-compose.yml` to:

- Run `go generate ./...` to build the frontend code
- Start the wiki application with `go run main.go`
- Optionally start additional services like Structurizr Lite for documentation

This is the recommended way to run and test the application during development.

Alternatively, you can use:

```bash
devbox services up
```

to start the process manager with all services.

#### Accessing the Application

The wiki will be running at <http://localhost:8050>, and you can access the Structurizr Lite documentation at <http://localhost:8080>.

### Running Tests

Tests for both frontend (JavaScript) and Go MUST be run using `devbox` scripts, ensuring a consistent environment.
Do not run them directly with `npx`, `npm`, `bun`, `bunx`, `go test`, etc. Use the provided scripts to ensure the setup is correct and that there are no environment issues causing false failures.

- **Frontend Tests (JavaScript)**:

  To run all frontend tests:

  ```bash
  devbox run fe:test
  ```

- **Go Tests**:

  To run all Go tests:

  ```bash
  devbox run go:test
  ```

### Robust Asynchronous Testing in JavaScript

Hanging or flaky async tests are common. The key is to ensure every asynchronous operation either resolves or rejects within a reasonable time. While a global test timeout is a good safety net, the following patterns help create more robust and predictable async tests.

#### 1. Stub Network and API Calls

This is the most frequent cause of hangs. If a component's setup code (e.g., in its `connectedCallback`) fetches data, the test will hang if the server is slow or unreachable.

**Pattern:** Use a library like `sinon` to intercept network requests and provide a fake, immediate response.

**Example:**

```typescript
// In your test file
import { stub } from "sinon";

describe("My Component", () => {
  let fetchStub;

  beforeEach(async () => {
    // Stub the global fetch function before the component is created
    fetchStub = stub(window, "fetch");
    // Make it resolve instantly with a fake response
    fetchStub.resolves(
      new Response(JSON.stringify({ id: 1, name: "Test Data" })),
    );

    // Now, when you create the component, it won't make a real network call
    const el = await fixture(html`<my-component></my-component>`);
  });

  afterEach(() => {
    // Clean up the stub after each test
    fetchStub.restore();
  });
});
```

#### 2. Control Time with Fake Timers

If your component uses `setTimeout`, `setInterval`, or other time-based functions, waiting for them in real-time makes tests slow and fragile.

**Pattern:** Use `sinon`'s fake timers to control the clock from your test.

**Example:**

```typescript
import { useFakeTimers } from "sinon";

describe("My Timed Component", () => {
  let clock;

  beforeEach(() => {
    // Take control of time
    clock = useFakeTimers();
  });

  afterEach(() => {
    // Give control back
    clock.restore();
  });

  it("should do something after 2 seconds", async () => {
    const el = await fixture(html`<my-timed-component></my-timed-component>`);

    // Fast-forward the clock by 2 seconds instantly
    await clock.tickAsync(2000);

    // Now you can assert what was supposed to happen after the delay
    expect(el.state).to.equal("loaded");
  });
});
```

#### 3. Use `Promise.race` for Explicit Timeouts

While the test runner timeout is a good safety net, you can add more specific timeouts to your setup logic to pinpoint exactly which async operation is failing.

**Pattern:** Race your async setup operation against a timeout promise. Whichever finishes first wins.

**Example:**

```typescript
function timeout(ms, message) {
  return new Promise((_, reject) =>
    setTimeout(() => reject(new Error(message)), ms),
  );
}

beforeEach(async () => {
  try {
    // This will fail if the fixture takes longer than 3 seconds
    el = await Promise.race([
      fixture(html`<frontmatter-editor-dialog></frontmatter-editor-dialog>`),
      timeout(3000, "Component fixture timed out"),
    ]);
  } catch (e) {
    // The error will clearly state that the fixture timed out
    console.error(e);
    throw e;
  }
});
```

- Prefer Context-Specification style for testing. Nest `describe` blocks to build up context. Don't bother with `context` blocks.
- **Separate test assertions**: Each `it` block should test one specific behavior. If a test description contains "and", it indicates the test is checking multiple behaviors and should be split into separate `it` blocks.

  **Bad:** Testing multiple behaviors in one block.

  ```javascript
  it('should handle rejection events and prevent default', () => {
    // This tests TWO behaviors: handling events AND preventing default
    expect(() => rejectionHandler(mockRejectionEvent)).to.not.throw();
    expect(preventDefaultStub).to.have.been.calledOnce;
  });
  ```

  **Good:** Split into separate, focused tests.

  ```javascript
  it('should handle rejection events', () => {
    expect(() => rejectionHandler(mockRejectionEvent)).to.not.throw();
  });

  it('should prevent default on rejection events', () => {
    rejectionHandler(mockRejectionEvent);
    expect(preventDefaultStub).to.have.been.calledOnce;
  });
  ```

- Don't do actions in the `It` blocks. The `It` blocks should only contain assertions. All setup (**Arrange**) and execution (**Act**) should be done in `BeforeEach` blocks (or equivalent, depending on the testing framework) within the `Describe` or `When` blocks. This allows for reusing context to add additional assertions later.

  **Bad:** Action inside the `It` block (Go example).

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

  **Good:** Action moved to `BeforeEach` (Go example).

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

  **Good:** Action moved to `beforeEach` (JavaScript Example `static/js/web-components/wiki-search.test.js`).

  ```javascript
  describe("when component is connected to DOM", () => {
    let addEventListenerSpy;

    beforeEach(async () => {
      addEventListenerSpy = sinon.spy(window, "addEventListener");
      // Re-create the element to trigger connectedCallback
      el = await fixture(html`<wiki-search></wiki-search>`);
      await el.updateComplete;
    });

    it("should add keydown event listener", () => {
      expect(addEventListenerSpy).to.have.been.calledWith(
        "keydown",
        el._handleKeydown,
      );
    });
  });
  ```

- When asserting an error, check for the specific error type or message. Do not just check that an error is not `nil`. This ensures that the test is validating the specific error that is expected to be returned.

  **Bad:** (Go example)

  ```go
  Expect(err).To(HaveOccurred())
  ```

  **Good:** (Go example)

  ```go
  Expect(err).To(MatchError("specific error message"))
  ```

- Use the `Describe` blocks first to describe the function/component being tested, then use nested `When` blocks to establish the scenarios. Besides the basic `It(text: "Should exist"` tests, everything should be in those nested "When" blocks.
- **Important**: Use "when" in `describe` block descriptions to establish scenarios, not in `it` block descriptions. The `it` blocks should describe the expected behavior or outcome.

  **Bad:**

  ```javascript
  it("should close when clicking outside", () => {
    // ... test code
  });
  ```

  **Good:**

  ```javascript
  describe("when clicking outside", () => {
    beforeEach(() => {
      // ... setup and action
    });

    it("should close the popover", () => {
      // ... assertion only
    });
  });
  ```

- **Event Handler Wiring Tests**: For web components that add/remove event listeners, always test that the event handlers are properly wired up. Use spies to verify that `addEventListener` and `removeEventListener` are called with the correct parameters and function references.

  **Example:**

  ```javascript
  describe("when component is connected to DOM", () => {
    let addEventListenerSpy;

    beforeEach(async () => {
      addEventListenerSpy = sinon.spy(document, "addEventListener");
      // Re-create the element to trigger connectedCallback
      el = await fixture(html`<my-component></my-component>`);
      await el.updateComplete;
    });

    it("should add event listener with correct parameters", () => {
      expect(addEventListenerSpy).to.have.been.calledWith(
        "click",
        el._handleClick,
      );
    });
  });
  ```

  This ensures the event listeners are properly registered and prevents memory leaks from incorrectly bound functions.

- **Comprehensive Testing**: Tests should validate the entire user-facing functionality, not just internal implementation details. When unit tests are green, we should be confident the whole app works. For example, when testing a search component, verify not only that results are processed correctly but also that the results view component becomes visible to the user.

- **No CSS-Only Testing**: Tests should not assert that specific CSS properties are applied unless it's for functional reasons (e.g., verifying visibility changes). Avoid testing computed styles like `getComputedStyle(element).property` unless the test verifies functional behavior that depends on that styling. Focus tests on component behavior, state management, event handling, and user interaction rather than visual styling details.

- **Documentation of Testing Principles**: When you discover important testing principles or patterns that ensure comprehensive coverage, document them in this CONVENTIONS.md file. This builds a comprehensive guide for future developers and helps maintain consistent testing practices across the project.

- Include a blank line between all the various Ginkgo blocks. This makes it easier to read the tests.

- Prefer Gomego/Ginkgo for testing in Go.
- Use a Context-Specification style. Nest `describe` blocks to build up context. Don't bother with `context` blocks in frameworks that provide them.
- Don't do actions in the `It` blocks. The `It` blocks should only contain assertions. All setup (**Arrange**) and execution (**Act**) should be done in `BeforeEach` blocks within the `Describe` or `When` blocks (`When` blocks if provided by the framework of course. Put "When" in the description of the `Describe` block if `When` blocks aren't available). This allows for reusing context to add additional assertions later.

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
- **Important**: Use "when" in `describe` block descriptions to establish scenarios, not in `it` block descriptions. The `it` blocks should describe the expected behavior or outcome.
- Include a blank line between all the various Ginkgo blocks. This makes it easier to read the tests.
- For a detailed checklist of test file conformance, refer to [Test File Conformance Checklist](docs/TEST_FILE_CHECKLIST.md).

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
- **Never Branch Logic on Error Messages**: Error messages are intended for human consumption and should never be used for conditional logic or control flow. Use proper error types, error codes, or structured error objects instead.

  **Bad (TypeScript):**

  ```typescript
  try {
    await client.getFrontmatter(request);
  } catch (err) {
    // Bad: Branching on human-readable error message
    if (err.message.includes("UNAVAILABLE")) {
      this.error = "Unable to connect to server";
    } else if (err.message.includes("PERMISSION_DENIED")) {
      this.error = "Access denied";
    }
  }
  ```

  **Good (TypeScript):**

  ```typescript
  import { ConnectError, Code } from "@connectrpc/connect";

  try {
    await client.getFrontmatter(request);
  } catch (err) {
    // Good: Using proper error codes for logic
    if (err instanceof ConnectError) {
      switch (err.code) {
        case Code.Unavailable:
          this.error = "Unable to connect to server";
          break;
        case Code.PermissionDenied:
          this.error = "Access denied";
          break;
        default:
          this.error = "An unexpected error occurred";
      }
    }
  }
  ```

  This approach ensures that:

  - Error logic remains stable when error message wording changes
  - Code is more maintainable and less fragile
  - Error handling is explicit and type-safe
  - Error messages can be localized without breaking logic

- **Centralize Error Processing**: Create dedicated error service classes to handle the conversion of technical errors (like gRPC errors) into user-friendly messages. This separates business logic from UI components and ensures consistent error presentation.

  **Bad:**

  ```typescript
  // Error message generation scattered across UI components
  class SomeComponent {
    async loadData() {
      try {
        // ... API call
      } catch (err) {
        if (err instanceof ConnectError && err.code === Code.NotFound) {
          this.error = "Data not found. Please check your input.";
        }
        // Duplicate error handling logic in every component
      }
    }
  }
  ```

  **Good:**

  ```typescript
  // Centralized error processing
  class ErrorService {
    static processError(error: unknown, context: string): ProcessedError {
      if (error instanceof ConnectError) {
        switch (error.code) {
          case Code.NotFound:
            return { message: "Data not found", icon: "not-found" };
          // ... other cases
        }
      }
      return { message: `Failed to ${context}`, icon: "error" };
    }
  }

  // Clean, consistent UI components
  class SomeComponent {
    async loadData() {
      try {
        // ... API call
      } catch (err) {
        const processedError = ErrorService.processError(err, "load data");
        this.error = processedError.message;
        this.errorIcon = processedError.icon;
      }
    }
  }
  ```

- **Never Hide Broken Functionality**: Do not make systems appear to work when they are actually broken. This includes:

  - Avoid showing fallback data that looks like real data when services are unavailable
  - Prefer clear error messages over misleading success states
  - Components should remain blank or show clear error states rather than fake data
  - **Don't return fake placeholder values**: Functions should return empty strings, nil, or appropriate zero values rather than fake data like "unknown" or "placeholder" that could mislead developers
  - **Be explicit about limitations**: If a function cannot provide the requested data, it should clearly indicate this through its return value and/or documentation
  - This principle helps identify real problems quickly and prevents false confidence in broken systems

  **Bad (TypeScript):**

  ```typescript
  function formatTimestamp(timestamp?: Timestamp): string {
    if (!timestamp) return "Unknown";
    try {
      return timestamp.toDate().toLocaleDateString();
    } catch {
      return "Invalid date"; // This hides the real problem
    }
  }
  ```

  **Good (TypeScript):**

  ```typescript
  function formatTimestamp(timestamp: Timestamp): string {
    // Let the function throw if timestamp is invalid - don't hide the error
    const date = timestamp.toDate();
    return date.toLocaleDateString();
  }

  // Usage - handle the null case at the call site
  const formatted = buildTime ? formatTimestamp(buildTime) : "";
  ```

- **Avoid Nullable Function Parameters**: Nullable parameters (`param?: Type`) should be rare for function parameters. It's preferable to force an exception at the source of the problem rather than handle null cases inside functions. This makes the code more predictable and helps identify issues earlier.

  **Bad (TypeScript):**

  ```typescript
  function processUser(user?: User): string {
    if (!user) return "No user";
    return user.name;
  }
  ```

  **Good (TypeScript):**

  ```typescript
  function processUser(user: User): string {
    return user.name;
  }

  // Handle the null case at the call site
  const result = user ? processUser(user) : "No user";
  ```

## Exception Handling Strategy

The application follows a **selective exception handling** strategy: **only catch exceptions you can meaningfully handle or recover from**. All unhandled exceptions should bubble up to the global error handler to provide a consistent user experience.

### Global Error Handler

The application has a global error handler that:

- Catches all unhandled JavaScript errors (`window.addEventListener('error')`)
- Catches all unhandled promise rejections (`window.addEventListener('unhandledrejection')`)
- Displays a kernel panic screen with error details processed through `ErrorService`
- Allows users to refresh and restart the application

This ensures that even unhandled errors provide a user-friendly experience rather than a broken application.

### When to Catch Exceptions

**✅ DO catch exceptions when:**

- You can provide meaningful user feedback and allow retry (e.g., network requests)
- You can gracefully degrade functionality while keeping the app usable
- You can recover automatically or provide alternative behavior
- The error is expected and part of normal operation (e.g., validation errors)

**❌ DON'T catch exceptions when:**

- You're just logging and re-throwing without handling
- The error represents a programming bug that should be fixed
- You can't provide any meaningful recovery or user action
- You're hiding genuine problems that should be addressed

### Exception Handling Patterns

**Good: Handle recoverable errors**

```typescript
// User can retry this operation
async loadData() {
  try {
    this.loading = true;
    this.error = undefined;
    const response = await this.client.getData();
    this.data = response;
  } catch (err) {
    // Process error and show user-friendly message
    const processedError = ErrorService.processError(err, 'load data');
    this.error = processedError.message;
    this.errorDetails = processedError.details;
    // User can click reload button to retry
  } finally {
    this.loading = false;
  }
}
```

**Good: Let unrecoverable errors bubble up**

```typescript
// This represents data corruption - nothing the component can do
private convertData(corruptedData: unknown): ProcessedData {
  if (!this.isValidData(corruptedData)) {
    // Don't catch this - let it bubble to global handler
    throw new Error('Data corruption detected in user profile');
  }
  return this.processData(corruptedData);
}
```

**Bad: Catching without meaningful handling**

```typescript
// Bad: Just logging and hiding the error
async saveDocument() {
  try {
    await this.client.save(this.document);
  } catch (err) {
    console.error('Save failed:', err); // Just logging
    // No user feedback, no retry mechanism, error is hidden
  }
}
```

### Error Recovery Guidelines

- **Provide clear user feedback** for all caught errors
- **Enable retry mechanisms** when operations can be reattempted
- **Maintain application state consistency** when errors occur
- **Use ErrorService.processError()** for consistent error presentation
- **Document recovery paths** in component interfaces and user documentation

### Testing Exception Scenarios

- **Test error paths** as thoroughly as success paths
- **Verify error messages** are user-friendly and actionable
- **Test global error handler** doesn't interfere with intentional error handling
- **Simulate network failures, timeouts, and server errors** in tests
- **Ensure error states can be recovered from** without full page refresh

This strategy ensures robust error handling while maintaining a clean separation between recoverable local errors and unrecoverable system errors.

### Running Linters

The project uses different linters for different parts of the codebase:

- **Go linting**: Use `devbox run go:lint` to run the Go linter (revive)
- **Frontend linting**: Use `devbox run fe:lint` to run the frontend TypeScript/JavaScript linter (ESLint)
- **All linting**: Use `devbox run lint:everything` to run all linters, tests, and builds

Each linter enforces specific rules:

- Go linter enforces using `any` instead of `interface{}` and other Go best practices
- Frontend linter enforces TypeScript strict typing with `@typescript-eslint/no-explicit-any` rule enabled

### Required Before Each Commit to Make Sure Everything Works

- Run the tests, builds, and linters with `devbox run lint:everything`.
- Run the application and ensure you can interact with it.
- Examine any recently written tests to ensure they conform to the testing guidance.

## Architecture Decision Records (ADRs)

ADRs should document significant architectural decisions that have long-term implications for the system design. Use ADRs for:

- **Significant architectural choices**: Technology stack decisions, database choices, communication patterns between services
- **Design patterns**: Adoption of specific architectural patterns (e.g., event sourcing, CQRS, microservices vs monolith)
- **Cross-cutting concerns**: Logging, monitoring, security, authentication strategies
- **Trade-offs with consequences**: Decisions where there are clear alternatives with different pros/cons
- **Decisions that could be questioned later**: Choices that future developers might wonder "why did we do it this way?"

**Do NOT create ADRs for:**

- Simple component implementations
- UI styling decisions
- Routine feature additions
- Standard library usage
- Minor refactoring decisions
- Implementation details that don't affect overall architecture

**Example of ADR-worthy decision**: "We chose gRPC-Web over REST for frontend-backend communication"  
**Example of non-ADR decision**: "We implemented a version display component in the bottom-right corner"

## README

- When updating the readme, match the tone of voice in the rest of the README. Its the face of the project. Marketing matters.
