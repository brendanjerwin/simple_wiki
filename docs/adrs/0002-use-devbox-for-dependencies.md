# ADR 0002: Use Devbox for Dependency Management

## Status

Accepted

## Context

As our project grows, we will introduce various command-line tools and dependencies, such as `go`, `buf`, and `nodejs`. Managing these dependencies across different developer machines and operating systems can lead to inconsistencies, version conflicts, and the "it works on my machine" problem. We need a reliable way to ensure that all developers use the same versions of the same tools.

## Decision

We will use [Devbox](https://www.jetpack.io/devbox/) by Jetify to create isolated, reproducible development environments.

- All project-level dependencies (e.g., `buf`, `go`, linters, etc.) will be managed via the `devbox.json` file.
- Developers will run `devbox shell` to activate the environment, which makes all the project's tools available in the `PATH`.
- New dependencies must be added using the `devbox add <package>` command. This ensures the `devbox.json` and `devbox.lock` files are updated correctly.

## Consequences

- **Pros**:
  - **Reproducibility**: Guarantees that all developers are using the exact same versions of tools, regardless of their host OS.
  - **Simplified Onboarding**: New developers can get started quickly by installing Devbox and running `devbox shell`.
  - **Version Control**: The `devbox.json` and `devbox.lock` files can be committed to version control, making the environment's configuration transparent and auditable.
  - **Isolation**: Project dependencies do not pollute the global system environment.
- **Cons**:
  - **New Tool**: Developers must install and learn how to use Devbox.
  - **Underlying Dependency**: Devbox relies on Nix, which must be installed on the developer's machine. This is an additional setup step.
