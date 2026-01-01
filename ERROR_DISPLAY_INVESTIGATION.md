# Error Display Pattern Investigation

**Date:** 2026-01-01  
**Issue:** Investigate error display patterns for CTA button consistency

## Executive Summary

This investigation examined all error display patterns across the Simple Wiki codebase to identify:
1. Which components are using the `error-display` component
2. Which components should be using it
3. Opportunities for the new CTA button feature

### Key Findings

- **6 components** currently use inline error rendering (custom error messages)
- **3 components** already use the `error-display` component correctly
- **5 opportunities** identified for adding CTA buttons to improve error recovery UX

---

## Components Using `error-display` Component ✅

These components correctly use the standardized `error-display` component:

### 1. `page-import-dialog.ts`
**Location:** Lines 887-888, 1107  
**Usage:** 
```typescript
${this.error ? html`<error-display .augmentedError=${this.error}></error-display>` : nothing}
```

**Status:** ✅ **Correctly using error-display**  
**Error Property Type:** `AugmentedError | null`  
**CTA Opportunity:** ⚠️ **Medium Priority**
- **Parsing errors** (line 887): Could add "Try Different File" button
- **Import errors** (line 1107): Could add "Retry" button for transient failures

---

### 2. `insert-new-page-dialog.ts`
**Location:** Lines 499-500  
**Usage:**
```typescript
${this.error ? html`<error-display .augmentedError=${this.error}></error-display>` : nothing}
```

**Status:** ✅ **Correctly using error-display**  
**Error Property Type:** `AugmentedError | null`  
**CTA Opportunity:** ⚠️ **Low Priority**
- Template loading errors: Could add "Retry" button
- Page creation errors: Most are validation errors (non-actionable)

---

### 3. `confirmation-dialog.ts`
**Location:** Line 4 (imports), used in component  
**Usage:** Imports and uses error-display component

**Status:** ✅ **Correctly using error-display**  
**Error Property Type:** `AugmentedError | null`  
**CTA Opportunity:** ❌ **None** - Dialog already has confirmation actions

---

### 4. `kernel-panic.ts`
**Location:** Lines 163-168  
**Usage:**
```typescript
${this.augmentedError ? html`
  <error-display 
    .augmentedError=${this.augmentedError}
    style="background: #330000; border-color: #660000; color: #ffcccc;">
  </error-display>
` : ''}
```

**Status:** ✅ **Correctly using error-display**  
**Error Property Type:** `AugmentedError | null`  
**CTA Opportunity:** ✅ **Already has CTA** - "Refresh Page" button (outside error-display)

---

## Components Using Inline Error Rendering ⚠️

These components use custom inline error rendering and should be evaluated for migration to `error-display`:

### 5. `wiki-search.ts`
**Location:** Line 241  
**Current Implementation:**
```typescript
${this.error ? html`<div class="error">${this.error.message}</div>` : ''}
```

**Status:** ⚠️ **Should migrate to error-display**  
**Error Property Type:** `Error | null`  
**Issues:**
- Uses plain `Error` instead of `AugmentedError`
- Custom CSS styling (`.error` class with background/border)
- Inconsistent with rest of application

**Recommendation:** 
- ✅ **Migrate to `error-display` component**
- Wrap errors with `AugmentErrorService.augmentError(error, 'searching')`
- Already using `coerceThirdPartyError` in lines 195, 226

**CTA Opportunity:** 🎯 **High Priority**
- **Search failures**: Add "Retry Search" button
- Network errors are common and retryable
- Improves user experience significantly

**Migration Effort:** Low - Already imports augment-error-service

---

### 6. `inventory-add-item-dialog.ts`
**Location:** Lines 420-422  
**Current Implementation:**
```typescript
${this.error ? html`<div class="error-message">${this.error.message}</div>` : ''}
```

**Status:** ⚠️ **Should migrate to error-display**  
**Error Property Type:** `Error | null`  
**Issues:**
- Uses plain `Error` instead of `AugmentedError`
- Custom CSS styling (`.error-message` class)
- Inconsistent error handling

**Recommendation:**
- ✅ **Migrate to `error-display` component**
- Change error property type to `AugmentedError | null`
- Use `AugmentErrorService.augmentError()` when setting errors

**CTA Opportunity:** ⚠️ **Low Priority**
- Most errors are validation errors (non-actionable)
- Could add "Retry" for network failures

**Migration Effort:** Low

---

### 7. `inventory-move-item-dialog.ts`
**Location:** Lines 685-687  
**Current Implementation:**
```typescript
${this.error ? html`<div class="error-message">${this.error.message}</div>` : ''}
```

**Additional Error Handling:** Lines 657-673 (scan errors with custom UI)
```typescript
private _renderScanError() {
  if (!this.scanError) {
    return nothing;
  }
  return html`
    <div class="scan-error">
      <div class="scan-error-message">
        <span class="icon"><i class="fa-solid fa-triangle-exclamation"></i></span>
        ${this.scanError.message}
      </div>
      <button class="scan-again-button" @click=${this._handleScanAgain}>
        <i class="fa-solid fa-qrcode"></i> Scan Again
      </button>
    </div>
  `;
}
```

**Status:** ⚠️ **Should migrate to error-display**  
**Error Properties:** 
- `error: Error | null` (line 686)
- `scanError: Error | null` (line 662)

**Issues:**
- Uses plain `Error` instead of `AugmentedError`
- Custom CSS styling for both error types
- Scan errors have inline CTA button (could use error-display CTA feature)

**Recommendation:**
- ✅ **Migrate to `error-display` component**
- Replace both error types with `AugmentedError`
- **Use CTA button feature** for scan errors instead of inline button

**CTA Opportunity:** 🎯 **High Priority - Already Has CTA!**
- **Scan errors**: Already has "Scan Again" button - perfect migration candidate
- **Move errors**: Could add "Retry" button for transient failures

**Migration Effort:** Medium - Two error types to convert

---

### 8. `qr-scanner.ts`
**Location:** Lines 623-625  
**Current Implementation:**
```typescript
${this.error ? html`<div class="error-message">${this.error.message}</div>` : nothing}
```

**Status:** ⚠️ **Should migrate to error-display**  
**Error Property Type:** `Error | undefined`  
**Issues:**
- Uses plain `Error` instead of `AugmentedError`
- Custom CSS styling (`.error-message` class)
- Good error handling (custom error types: `CameraPermissionError`, `NoCameraError`)

**Recommendation:**
- ✅ **Migrate to `error-display` component**
- Convert custom error types to work with `AugmentedError`
- Map error types to appropriate icons

**CTA Opportunity:** 🎯 **High Priority**
- **Camera permission errors**: Add "Open Settings" or "Grant Permission" guidance
- **No camera errors**: Add "Check Device" button or guidance link
- **Scanner errors**: Add "Retry" button

**Migration Effort:** Medium - Custom error types need integration

---

### 9. `system-info-version.ts`
**Location:** Line 107  
**Current Implementation:**
```typescript
private renderError() {
  return html`<div class="error">${this.error?.message}</div>`;
}
```

**Status:** ❌ **Should NOT migrate** (special case)  
**Error Property Type:** `Error | null`  
**Reason:** 
- Compact system info widget in corner of screen
- Limited space for error display
- Simple error messages appropriate for context
- Not a primary user interaction point

**CTA Opportunity:** ❌ **None** - Space constrained, non-critical errors

---

### 10. `system-info-indexing.ts`
**Location:** Line 150 (only partial view shown)  
**Status:** ⏭️ **Needs fuller examination**  
**Note:** File was only partially examined. May have error rendering patterns.

---

### 11. `system-info.ts`
**Location:** Lines 129, 148  
**Properties:**
```typescript
declare error: Error | null;
```

**Status:** ❌ **Should NOT migrate** (special case)  
**Reason:** Same as system-info-version - compact widget, limited space

**CTA Opportunity:** ❌ **None** - Space constrained

---

## Summary of Recommendations

### High Priority Migrations

#### 1. `wiki-search.ts` 🎯
- **Migrate to:** `error-display` component
- **Add CTA:** "Retry Search" button
- **Impact:** High user visibility, common error scenario
- **Effort:** Low

#### 2. `inventory-move-item-dialog.ts` 🎯
- **Migrate to:** `error-display` component  
- **Add CTA:** "Scan Again" button (replace inline button)
- **Impact:** Already has CTA, makes it consistent
- **Effort:** Medium (two error types)

#### 3. `qr-scanner.ts` 🎯
- **Migrate to:** `error-display` component
- **Add CTA:** "Retry" / "Check Permissions" buttons
- **Impact:** Improves camera error recovery
- **Effort:** Medium (custom error types)

### Medium Priority Migrations

#### 4. `inventory-add-item-dialog.ts` ⚠️
- **Migrate to:** `error-display` component
- **Add CTA:** "Retry" button (for network errors)
- **Impact:** Improves consistency
- **Effort:** Low

#### 5. `page-import-dialog.ts` (already using, add CTA) ⚠️
- **Already uses:** `error-display` ✅
- **Add CTA:** "Try Different File" / "Retry" buttons
- **Impact:** Improves error recovery for imports
- **Effort:** Low (component already integrated)

### Low Priority

#### 6. `insert-new-page-dialog.ts` (already using, minimal CTA benefit)
- **Already uses:** `error-display` ✅
- **Potential CTA:** "Retry" for template loading
- **Impact:** Low (most errors are validation)
- **Effort:** Low

---

## CTA Button Opportunities by Error Type

### Network/Transient Errors
- ✅ **Retry action** - Most common CTA need
- Components: `wiki-search`, `page-import-dialog`, `inventory-add-item-dialog`

### Permission/Setup Errors  
- ✅ **Grant Permission** / **Open Settings** guidance
- Components: `qr-scanner`

### Scanning/Camera Errors
- ✅ **Scan Again** / **Try Different Camera**
- Components: `inventory-move-item-dialog`, `qr-scanner`

### File/Import Errors
- ✅ **Try Different File** / **Download Template**
- Components: `page-import-dialog`

### Navigation/Fallback Actions
- ✅ **Go Back** / **Return to Home** - Not currently needed
- ✅ **Switch to Manual Mode** - Not currently needed

---

## Implementation Guide

### Step 1: Migrate Component to Use `error-display`

**Before:**
```typescript
${this.error ? html`<div class="error-message">${this.error.message}</div>` : ''}
```

**After:**
```typescript
import './error-display.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';

// Change error property type
declare error: AugmentedError | null;

// In render method
${this.error ? html`<error-display .augmentedError=${this.error}></error-display>` : ''}
```

### Step 2: Add Error Augmentation When Setting Errors

**Before:**
```typescript
this.error = new Error('Something went wrong');
```

**After:**
```typescript
this.error = AugmentErrorService.augmentError(
  new Error('Something went wrong'),
  'performing action'  // Failed goal description
);
```

### Step 3: Add CTA Button

```typescript
import type { ErrorAction } from './error-display.js';

// Create action handler
private _handleRetry = (): void => {
  this.error = null;
  this._performAction();  // Retry the failed action
};

// In render method
${this.error ? html`
  <error-display 
    .augmentedError=${this.error}
    .action=${{
      label: 'Retry',
      onClick: this._handleRetry
    } satisfies ErrorAction}
  ></error-display>
` : ''}
```

---

## Files to Update

### High Priority
1. `static/js/web-components/wiki-search.ts` - Add error-display + Retry CTA
2. `static/js/web-components/inventory-move-item-dialog.ts` - Add error-display + Scan Again CTA  
3. `static/js/web-components/qr-scanner.ts` - Add error-display + camera error CTAs

### Medium Priority
4. `static/js/web-components/inventory-add-item-dialog.ts` - Add error-display
5. `static/js/web-components/page-import-dialog.ts` - Add CTAs to existing error-display

### Low Priority
6. `static/js/web-components/insert-new-page-dialog.ts` - Add CTAs to existing error-display

---

## Testing Checklist

For each migrated component:
- [ ] Error displays with correct icon
- [ ] Error message is readable and helpful
- [ ] CTA button appears when error is actionable
- [ ] CTA button triggers correct action
- [ ] Error clears after successful retry
- [ ] Keyboard accessibility works (Tab, Enter)
- [ ] Screen reader announces error correctly
- [ ] Visual styling matches design system

---

## Next Steps

1. **Create follow-up issues** for each high-priority migration
2. **Implement migrations** in order of priority
3. **Add tests** for CTA button interactions
4. **Update Storybook** stories to demonstrate error states with CTAs
5. **Document patterns** in component library for future development

---

## Related Files

- `static/js/web-components/error-display.ts` - The error display component
- `static/js/web-components/error-display.test.ts` - Component tests
- `static/js/web-components/error-display.stories.ts` - Storybook stories
- `static/js/web-components/augment-error-service.ts` - Error augmentation service
