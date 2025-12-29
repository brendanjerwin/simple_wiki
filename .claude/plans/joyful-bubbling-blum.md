# Fix inventory-qr-scanner protobuf v2 compatibility and error display

## Problem Summary

1. **`toJson()` error**: After protobuf-es v2 upgrade (PR #214), `response.frontmatter?.toJson()` fails because in v2, `frontmatter` is already a `JsonObject` - no conversion needed.

2. **Custom error display**: Component uses inline error display instead of standard `error-display` component with CTA support (per issue #219).

## Root Cause

In protobuf-es v2, `google.protobuf.Struct` fields are typed directly as `JsonObject`:
```typescript
// static/js/gen/api/v1/frontmatter_pb.ts:47
frontmatter?: JsonObject;
```

The old v1 code called `.toJson()` on a Struct message object. In v2, it's already a plain object.

## Files to Modify

1. **`static/js/web-components/inventory-qr-scanner.ts`**
   - Line 207: Remove `.toJson()` call - just use `response.frontmatter` directly
   - Lines 85-117: Remove custom error CSS
   - Lines 255-265: Replace inline error template with `<error-display>` component
   - Add import for `error-display.js` and `AugmentErrorService`
   - Change `error` property type from `Error | null` to `AugmentedError | null`
   - Add CTA action for "Scan Again" button

2. **`static/js/web-components/inventory-qr-scanner.test.ts`**
   - Line 18: Update mock to return plain object instead of object with `toJson()` method
   - Add tests for error-display integration

## Implementation Steps

### Step 1: Fix the toJson error
```typescript
// Before (line 207):
const fm = response.frontmatter?.toJson() as Record<string, unknown> | undefined;

// After:
const fm = response.frontmatter as Record<string, unknown> | undefined;
```

### Step 2: Add imports
```typescript
import './error-display.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
```

### Step 3: Change error property type
```typescript
@state()
private augmentedError: AugmentedError | null = null;
```

### Step 4: Update error handling in catch block
```typescript
} catch (err) {
  this.augmentedError = AugmentErrorService.augmentError(
    err instanceof Error ? err : new Error(`Page "${identifier}" not found`),
    'validating scanned page'
  );
}
```

### Step 5: Replace error template with error-display
```typescript
${this.augmentedError ? html`
  <error-display
    .augmentedError=${this.augmentedError}
    .action=${{
      label: 'Scan Again',
      onClick: () => this._handleScanAgain()
    }}
  ></error-display>
` : html`...`}
```

### Step 6: Remove custom error CSS (lines 85-117)

### Step 7: Update tests
- Remove `toJson` mock method
- Add tests verifying error-display is rendered on error
