## General

- prefer modern go. Update idioms to modern approaches as you see them. Boyscout rule.
- prefer IoC approaches. Make \*-er interfaces for all the things!

## Testing

- Prefer Gomego/Ginkgo for testing. Context-Specification style. Nest `describe` blocks to build up context. Don't bother with `context` blocks.
- Don't do actions in the `It` blocks. Do them as `beforeEach` blocks in the `Describe` blocks. We want to reuse context to add additional assertions later.
- Use the `Describe` blocks first to describe the function/component being tested, then use nested `When` blocks to establish the scenarios. Besides the basic `It(text: "Should exist"` tests, everything should be in those nested "When" blocks.
