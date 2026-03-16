Deploy the current branch or a specified ref to the wiki server.

## Instructions

1. Determine what to deploy:
   - If an argument is provided (e.g., `/deploy v3.4.0` or `/deploy feature/my-branch`), use that as the ref.
   - If no argument is provided, deploy the current branch.

2. **Main branch requires a GitHub Release, not just a tag.** Direct deployment of `main` is blocked by the deploy script. If on `main` or asked to deploy main:
   - Check the latest tags: `git tag --sort=-v:refname | head -5`
   - Check commits since the last tag: `git log <last-tag>..HEAD --oneline`
   - Determine the next version number (bump minor for features, patch for fixes)
   - Create a **GitHub Release** (not just a tag) with auto-generated release notes:
     ```
     gh release create vX.Y.Z --generate-notes --title "vX.Y.Z"
     ```
   - Wait for the Release workflow to build assets, then deploy the tag:
     ```
     devbox run deploy vX.Y.Z
     ```

3. **Feature branches deploy directly.** For any non-main branch:
   - Ensure all changes are committed (the script checks this)
   - Run: `devbox run deploy`
   - The script pushes the branch and triggers the deploy workflow automatically

4. The deploy script triggers a GitHub Actions workflow and watches it to completion. Report the result to the user.

## Quick Reference

- **Deploy current feature branch:** `devbox run deploy`
- **Deploy a specific branch:** `devbox run deploy feature/blog-macro`
- **Deploy main (requires release):** `gh release create vX.Y.Z --generate-notes --title "vX.Y.Z"` then `devbox run deploy vX.Y.Z`
