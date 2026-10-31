# #1320 Deferred Wave D — Graphics Primitives (Sketch / Canvas2D / SVG)

Deferral record mandated by PLAN-1320 (Wave D scope-gate). Written 2026-10-31 at run close of the wiki-core@3 component campaign.

## What shipped (all merged to `main`)

| Family | PR | Contents |
|---|---|---|
| P3 form controls | #1351, #1354, #1355 | Checkbox/Radio, mobile composer fix, Slider, NumberInput, Textarea, DatePicker |
| P4 display | #1357 | Badge, Icon, Avatar |
| P4 display (cont.) | #1360, #1361 | Skeleton, List |
| P4 imagery | #1358 | Card `backgroundImage` backdrop (validated URL allowlist) |
| P5 image | #1362 | `Image(src, alt?, w?, h?, fit?)` — validated upload/https src |

## What is deliberately deferred

`Sketch(kind)` and the P6 tier-1 `Canvas2D` / `SVG` declarative primitives
(PLAN-1320 Wave D / P6 tiers, plus PLAN-1302 graphics ladder) are **out of this
campaign's scope**:

1. No current page-app scenario in the live wiki calls for raw graphics
   primitives — all shipped surfaces are satisfied by the merged families
   above.
2. Graphics primitives need a concrete consumer before their API can be
   contract-shaped (e.g. `Canvas2D(commands, w?, h?)` vs a declarative
   primitive list like `Rect`, `Circle`, `Path`); building without one risks
   designing the wrong vocabulary — the same trap PLAN-1302 warns about for
   feature-request-driven components elsewhere.
3. Lottie / regl tiers (PLAN-1302 graphics ladder tier-2/3) stay blocked behind
   tier-1.

## Reopen condition

Reopen when a real page-app *needs* a drawn/graph surface — the same
"concrete app → issue" path the rest of the plan uses. At that point revisit
`PLAN-1302.md` graphics tiers side-by-side with whatever API the real app
suggests.

---
*Receipt runs: plan-vs-receipts review (`msg_9d312bf42033`) flagged the missing
deferral doc; this file is that record.*
