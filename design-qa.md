# Model Recommendations Responsive Design QA

## Comparison Target

- Source visual truth: `/var/folders/jn/786z2s3124bc9bm03k75l_r80000gn/T/codex-clipboard-b1205835-47b1-422d-a8bb-2dfaabf1e522.png` (`4490 x 1740` pixels).
- Implementation screenshot: `/private/tmp/sub2api-intelligence-2048x768.png` (`2040 x 765` pixels).
- Mobile screenshot: `/private/tmp/sub2api-intelligence-375x812.png` (`375 x 812` pixels).
- Combined comparison: `/private/tmp/sub2api-intelligence-comparison.png`.
- Desktop CSS viewport: `2048 x 768`; mobile CSS viewport: `375 x 812`; device scale factor 1.
- State: light theme, populated recommendation data, score-rail mode selected.

## Full-View Comparison

- The source demonstrates the issue: two wide columns allow the IQ rails to expand across most of an ultra-wide screen.
- The implementation preserves the existing card hierarchy and styling while switching the intelligence groups to three columns at the `xl` breakpoint.
- The recommendation grid stops at `96rem`, and each rail track stops at `15rem`, so neither continues growing with an ultra-wide viewport.
- At `375px`, the layout collapses to one column, the rail track measures `131px`, and document `scrollWidth` equals `clientWidth`.

## Focused Region Evidence

- The desktop intelligence section is readable in `/private/tmp/sub2api-intelligence-2048x768.png`; the first row contains three model cards with consistent widths.
- The mobile intelligence section is readable in `/private/tmp/sub2api-intelligence-375x812.png`; model name, effort, IQ, price, and time remain visible without horizontal overflow.
- A separate crop was not needed because both screenshots show the affected rail geometry and labels at readable scale.

## Required Fidelity Surfaces

- Fonts and typography: existing model, effort, IQ, price, and time typography is unchanged; labels remain legible and do not overlap.
- Spacing and layout rhythm: desktop uses three equal columns with the existing `12px` gaps; tablet uses two columns; mobile uses one column. Card and row padding remain unchanged.
- Colors and visual tokens: existing low-saturation model colors, borders, IQ bars, and dark-mode tokens are unchanged.
- Image and asset fidelity: this section contains no raster imagery. Existing icon components and model marks are preserved.
- Copy and content: recommendation labels, model names, effort levels, IQ values, prices, and durations are unchanged.

## Interaction And Responsive Checks

- Score-rail mode remains the default.
- Switching to compact matrix mode renders matrix cells and removes rail rows; switching back restores the rail rows.
- Measured desktop at `2048px`: three columns, `504px` cards, `228px` rails, and a `1536px` capped grid.
- Measured tablet at `1100px`: two columns, `491px` cards, and `215px` rails.
- Measured mobile at `375px`: one column, `301px` cards, `131px` rails, and no horizontal overflow.
- The mobile rail uses `minmax(0, 15rem)` so narrower supported screens can continue shrinking proportionally.
- Console inspection found only expected public-settings fetch failures from the frontend-only QA harness; no component rendering error was present.

## Comparison History

### Iteration 1

- Finding: [P2] two-column ultra-wide layout produced excessively long score rails.
- Fix: intelligence groups now use three columns from the `xl` breakpoint, the group grid is capped at `96rem`, and rail tracks are capped at `15rem`.
- Post-fix evidence: `/private/tmp/sub2api-intelligence-2048x768.png` and `/private/tmp/sub2api-intelligence-comparison.png`.

### Iteration 2

- Finding: [P2] the first mobile implementation retained a `7rem` minimum rail width, which could prevent proportional shrinking below `375px`.
- Fix: the mobile rail minimum is now `0` while retaining the `15rem` maximum.
- Post-fix evidence: responsive grid math plus the `375px` browser capture; final automated checks cover the production component.

## Findings

No actionable P0, P1, or P2 findings remain.

## Open Questions

None.

## Implementation Checklist

- [x] Three-column desktop layout.
- [x] Fixed maximum rail and grid widths.
- [x] Two-column tablet and one-column mobile fallbacks.
- [x] Proportional rail shrinking on narrow screens.
- [x] Mode switch and console checks.

## Follow-up Polish

No blocking or requested polish remains.

final result: passed
