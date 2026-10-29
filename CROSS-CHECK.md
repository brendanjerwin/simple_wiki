# Cross-check evidence: already-implemented wiki-core@3 components (issue #1320)

Source: verification scout (Orca dispatch `ctx_e38c016bdf39`, task `task_8acd4a3e3b64`), 2026-10-29. Recovered from `history://` transcript artifact after terminal release.

## Scan result: SUCCESS — shipped surface intact

- **Intact**: zero code changes to compiled component/enumeration files since the three shipped PRs — diff of `static/js/openui-lang/{library.ts,page-app-design-system.ts,library.test.ts}` between the last shipped commit (`a20d0bdc`) and HEAD is empty; all 8 components (Tabs, Accordion, Switch, Select, Dialog, Toast, Drawer, Tooltip) have live `defineComponent` definitions with renderers, children-union refs, and design-system CSS blocks intact.
- **Re-ran all five component e2e specs, scoped, serially (23/23 PASS, 0 FAIL)**; unit suite (`devbox run fe:test -- library.test.ts`) also green.
- **Docs render**: `help_page_apps.md` contains all 8 sections (Drawer/Dialog/Toast variants at lines 81-119, Form controls 123-145, Navigation 147-169, Tooltips 189-207); no new components added.

## Per-spec e2e results (five shipped-component specs, serial, scoped)

| spec | PASS/FAIL |
|------|-----------|
| page-app-components-dialog-toast | 6/6 |
| page-app-components-drawer | 3/3 |
| page-app-components-switch-select | 5/5 |
| page-app-components-tabs-accordion | 5/5 |
| page-app-components-tooltip | 4/4 |
| **Total** | **23/0** |

## Doc-drift findings (from same scout)

- **Duplicate Overlays headings** in `help_page_apps.md`: one at line 81 (`## Overlays: Dialog, Toast, Drawer`) and one at line 92 (`## Overlays: Dialog, Toast`) — merge drift between PR #1322 and #1326. Fix task `task_b1702423e6cb` scheduled post-merge: deduplicate into one section covering all three components, preserving every Drawer/Dialog/Toast example.
- **ADR-0017 sentence drift**: line 48 says "wiki-core@3 adds the P1 layout vocabulary" with no mention of the newer P3 components — refresh in the docs wave, not a code defect.

## Shipped-PR lineage

- #1322 — P1 remainder + P2 overlays + Switch/Select docs (Tabs, Accordion, Switch, Select, Dialog, Toast, Drawer)
- #1326 + #1329 — Tooltip (pure-CSS hover hint + flex shrink-wrap fix)

All shipped in v3.98.0, deployed and live-verified.
