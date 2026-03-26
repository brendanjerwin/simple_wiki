# ADR-0008: Drawer UX Layer System

## Status

Accepted

## Context

Multiple fixed-position UI elements (system-info panel, chat panel, FAB button) compete for screen space with no formal rules governing overlap, yielding, or coordination. Z-index values were ad-hoc (scattered from 998 to 10000), and the system-info panel at z-index 1000 covered the chat panel's textarea at z-index 998.

Rather than another z-index band-aid, we needed a formal UX system with encodable rules that prevent this class of problem.

## Decision

We introduce a layer system with four named UI concepts and a coordinator that enforces their rules.

### UI Concepts

| Concept | Definition | Examples |
|---------|-----------|----------|
| **Drawer** | Panel that slides from the right (desktop) or bottom (mobile) over content | system-info, chat panel |
| **Ambient-CTA** | Always-visible trigger for opening a drawer; hidden when any drawer is open | FAB button, INFO tab |
| **Notification** | Transient, non-interactive overlay | toast-message |
| **Blocker** | Full-screen element demanding immediate attention | kernel-panic, dialog modals |

### Rules

1. Only one drawer may be open at a time (mutual exclusion).
2. When any drawer opens, ALL Ambient-CTAs hide.
3. When all drawers close, all Ambient-CTAs show.
4. Drawers do not know about each other; they communicate only through the coordinator.
5. Desktop (>=768px): drawers slide from the right.
6. Mobile (<768px): drawers slide up from the bottom as sheets.

### Layer System (z-index tokens)

Semantic z-index tokens defined in `shared-styles.ts` as CSS custom properties:

```
--z-ambient: 100       (Ambient-CTAs)
--z-drawer: 200        (Slide-over panels)
--z-popover: 300       (Menus, tooltips)
--z-modal: 400         (Dialogs with backdrop)
--z-notification: 500  (Toasts)
--z-blocker: 600       (Kernel panic)
```

Within each layer, the order doesn't matter because the UX rules prevent overlap (e.g., only one drawer is open at a time).

### Implementation

- **DrawerCoordinator** (`drawer-coordinator.ts`): Singleton module managing drawer registration, mutual exclusion, and CTA visibility. Drawers and CTAs register/deregister independently.
- **DrawerMixin** (`drawer-mixin.ts`): Lit mixin providing `drawerOpen` state, `openDrawer()`/`closeDrawer()`/`toggleDrawer()` methods, and automatic coordinator registration.
- **Shared drawer CSS** (`drawer-styles.ts`): Base slide animation (right on desktop, bottom on mobile).

### Adding a New Drawer

1. Apply `DrawerMixin` to your component class.
2. Set a unique `drawerId`.
3. If your component has an Ambient-CTA trigger, implement the `AmbientCTA` interface and register with `registerAmbientCTA`.
4. Use `var(--z-drawer)` and `var(--z-ambient)` for z-index values.
5. The coordinator handles all coordination automatically.

## Consequences

- New drawers can be added by applying the mixin and registering — no z-index coordination needed.
- Components don't know about each other; only the coordinator.
- Z-index values are centralized in semantic tokens.
- The mutual exclusion rule is enforced automatically, preventing the overlap problem that motivated this change.
- Existing non-drawer fixed elements (toast, kernel-panic, modals) should migrate to the token system but don't need the coordinator.
