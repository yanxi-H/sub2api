# Monitor Center Design QA

## Comparison Target

- Source visual truth: `/Users/yuan/Downloads/sub2api_operations_center_cockpit_apple(1).html`
- Implementation: `http://127.0.0.1:3000/admin/monitor-center`
- Primary state: Chinese, light theme, 1-hour range, populated monitoring data, administrator session.
- Primary CSS viewport: `1440 x 900` at device scale factor 1.
- Source capture: `/Users/yuan/Desktop/sub2api/design-qa-source-final-1440.png` (`1425 x 891` visible page pixels after scrollbar exclusion).
- Implementation capture: `/Users/yuan/Desktop/sub2api/design-qa-implementation-final-1440.png` (`1432 x 895` visible page pixels after scrollbar exclusion).
- Density normalization: browser screenshots were captured at CSS-pixel density; the small pixel-size difference is only each page's scrollbar gutter.

## Full-View Comparison

- Information architecture matches the source: compact toolbar, cockpit, six resource metrics, separated OpenAI/gateway/real-probe health, latency chart, three concurrency lanes, slow-request diagnostics, and three-day history.
- The implementation intentionally uses the existing Sub2API admin sidebar, header, theme, permissions, and 8px surface radius instead of recreating the prototype shell. This is the requested product integration, not design drift.
- Dynamic values and empty/unknown states come from the application API contracts rather than the prototype's static demo values.
- Desktop hierarchy and density remain faithful: the cockpit and status sections fit above the latency chart at 1440px, and all major charts retain enough height for spike and recovery inspection.

## Focused Region Evidence

- Latency and concurrency: `/Users/yuan/Desktop/sub2api/design-qa-source-mid-1440.png` and `/Users/yuan/Desktop/sub2api/design-qa-implementation-mid-1440.png`.
- Concurrency and slow diagnostics: `/Users/yuan/Desktop/sub2api/design-qa-source-lower-1440.png` and `/Users/yuan/Desktop/sub2api/design-qa-implementation-lower-1440.png`.
- Slow-impact table and three-day history: `/Users/yuan/Desktop/sub2api/design-qa-source-bottom-1440.png` and `/Users/yuan/Desktop/sub2api/design-qa-implementation-bottom-1440.png`.
- Tablet: `/Users/yuan/Desktop/sub2api/design-qa-source-tablet-1024.png` (`1009 x 887`) and `/Users/yuan/Desktop/sub2api/design-qa-implementation-tablet-1024.png` (`1015 x 892`) at a `1024 x 900` CSS viewport.
- Mobile: `/Users/yuan/Desktop/sub2api/design-qa-source-mobile-390.png` (`375 x 812`) and `/Users/yuan/Desktop/sub2api/design-qa-implementation-mobile-390-fixed.png` (`382 x 827`) at a `390 x 844` CSS viewport.
- Dark theme: `/Users/yuan/Desktop/sub2api/design-qa-implementation-dark-1440.png`.

## Required Fidelity Surfaces

- Fonts and typography: both use the Apple system stack with PingFang SC fallbacks, zero letter spacing, compact operational labels, tabular numeric metrics, and restrained heading sizes. No clipped or overlapping text was observed.
- Spacing and layout rhythm: section order and dense dashboard rhythm match the source. The implementation uses existing admin gutters and shell dimensions; tablet grids collapse predictably and mobile modules become single-column without hiding persistent controls.
- Colors and tokens: neutral white/light-gray surfaces, low-saturation green/orange/red status colors, system blue interactions, and stable chart colors match the source intent. Dark-mode tokens preserve semantic distinctions without glow or gradients.
- Image and icon fidelity: the source contains no required raster product imagery. The implementation keeps the real Sub2API logo and uses one consistent icon library for UI controls; charts are rendered from real data rather than placeholder artwork.
- Copy and content: Chinese and English strings are localized. Static copy describes operational meaning; prototype-only demo wording and values are not shipped.

## Interaction Checks

- E2E and TTFT tabs switch the five latency series and expose `role=tab` plus `aria-selected`.
- Concurrency user selection changes only the user series; the internal refresh control remains available.
- Slow-impact user/account/model tabs update table headers and rows independently.
- Custom range rejects start-after-end and ranges longer than 30 days, accepts valid input, and refreshes without clearing previous successful data.
- 1h/6h/24h selection, global refresh, light/dark theme, tablet, and mobile layouts were exercised.
- Mobile implementation has no document-level horizontal overflow (`scrollWidth = clientWidth = 382px`).
- Console errors were checked. The only errors came from the temporary mock server's announcement payload and the existing compliance-dialog mock state; no Monitor Center runtime error was observed.

## Comparison History

### Iteration 1

- Finding: [P2] three-day history samples used fixed minimum column widths on mobile, producing document width `400px` inside a `390px` viewport.
- Fix: history bands now receive an explicit repeat column count with `minmax(0, 1fr)` and clip their own contents.
- Post-fix evidence: `/Users/yuan/Desktop/sub2api/design-qa-implementation-mobile-390-fixed.png`; measured document `scrollWidth` and `clientWidth` are both `382px` after scrollbar exclusion.
- Finding: [P2] visual tab state was not exposed to assistive technology.
- Fix: latency and slow-impact tab controls now use tab roles, `aria-selected`, and roving `tabindex`; time-range buttons expose `aria-pressed`.
- Post-fix evidence: browser accessibility inspection reports E2E selected and TTFT unselected, with user/account/model discoverable as tabs.

## Findings

No actionable P0, P1, or P2 findings remain.

## Open Questions

None. The implementation intentionally improves the prototype's mobile overflow and follows the current Sub2API admin shell rather than copying the standalone prototype chrome.

## Implementation Checklist

- [x] Desktop visual comparison at 1440 x 900.
- [x] Focused chart, concurrency, slow-request, and history comparisons.
- [x] Tablet, mobile, and dark-mode checks.
- [x] Primary controls and validation states exercised.
- [x] Console inspected and implementation-specific errors ruled out.

## Follow-up Polish

No blocking or requested polish remains.

final result: passed
