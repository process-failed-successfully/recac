## 2026-07-13 - [Fix UI Crash & Enhance Action Affordances]
**Learning:** Found that `replace` string operations on Go numeric variables (`int` passed to JS) crash DOM rendering (causing the features table to vanish and show a fake network error). Also learned that embedded "Retry" and "Refresh" action buttons in empty states were difficult to scan quickly.
**Action:** Always wrap integer-based properties in `String()` before performing string methods like `.replace()`. Also, prepended visually distinct icons (`<span aria-hidden="true">🔄</span> `) to repetitive interactive table/graph recovery actions for better scannability.

## 2026-07-14 - [Graceful Degradation for JS-Driven Interfaces]
**Learning:** Found that purely client-side rendered dashboards trapped non-JS users (or users on failing networks) in an infinite "Loading..." state because the DOM elements containing spinners are baked into the static HTML, relying entirely on JS to clear them.
**Action:** Implemented a `<noscript>` block inside the main layout containing a scoped `<style>` that sets `display: none !important` on interactive containers and shows a structured error state. This ensures a graceful, accessible fallback (WCAG 1.1.1 fallback content) rather than a broken UI.

## 2026-07-15 - Data Table Accessibility: Row Headers and aria-live
**Learning:** Using `aria-live="polite"` on a large container like a dynamically refreshing `tbody` causes excessive screen reader verbosity. Furthermore, dynamically populated table data cells can be made significantly more navigable horizontally by turning the first cell into `<th scope="row">` instead of `<td>`, provided its visual styling (like `background-color`) is appropriately adjusted to match `td` styles rather than default header styles.
**Action:** When working on dynamic data tables, avoid placing `aria-live` on the table body itself. Instead, rely on smaller, targeted status messages (like a "Last updated" text). Additionally, always convert the primary column cell (e.g., ID or Name) into a row header (`<th scope="row">`) to improve table structural navigation for assistive technology users.

## 2026-07-16 - [Exposing Keyboard Shortcuts to Screen Readers]
**Learning:** Found that visual keyboard shortcut hints (like `<kbd>R</kbd>`) were hidden from screen readers using `aria-hidden="true"` to prevent redundant/confusing reading ("Refresh R"). However, this left screen reader users completely unaware that a global keyboard shortcut existed for the action.
**Action:** When implementing visual keyboard shortcuts, always pair the visually hidden `<kbd>` tags with the `aria-keyshortcuts` attribute on the interactive element (e.g., `aria-keyshortcuts="r"`) so that assistive technologies can semantically announce the available shortcut.

## 2026-07-17 - Data Table UX: List Counts
**Learning:** Found that long data lists or dynamic tables without visible item counts make it difficult for users to quickly gauge system state or workload size without scrolling.
**Action:** When working on dynamic data lists or tables, add an inline item count badge to the container's primary heading using visually distinct styling (e.g. `.category-badge`), and ensure it remains accessible by including an `sr-only` prefix (e.g., "Total Tasks: ") and a `title` attribute for native hover tooltips.

## 2026-07-18 - [Transient Action Accessibility]
**Learning:** Found that simply changing the `aria-label` of an element after it is focused (like a Copy button changing to 'Copied') does not reliably trigger screen readers to announce the new text, leaving users unaware of the success or failure of transient actions.
**Action:** Implemented a dedicated `aria-live="polite"` announcer region (`<div id="a11y-announcer" class="sr-only">`) to explicitly push transient success/error messages to screen readers when asynchronous actions (like copying to clipboard) complete, clearing it after a timeout.

## 2026-07-19 - [Off-page Visibility via Document Title]
**Learning:** Found that users who monitor dashboards in background tabs miss critical system updates or error states because there is no off-page visibility. Relying purely on in-page notifications assumes the user always has the tab focused.
**Action:** When designing dashboards that poll or refresh data, dynamically update the `document.title` to reflect the active workload (e.g., prefixing with a task count `(5) Dashboard`) or critical error states (e.g., `⚠️ Error - Dashboard`), ensuring users can monitor the tab at a glance.

## 2026-07-20 - Responsive Data Table Legibility
**Learning:** Found that combining `table-layout: fixed` with responsive wrappers (`overflow-x: auto`) causes the table to crush content vertically on narrow viewports rather than triggering the intended horizontal scroll, unless the table element itself has a minimum width.
**Action:** When implementing responsive data tables, always set a `min-width` (e.g., `600px`) on the table element to guarantee structural legibility and force the wrapper to overflow horizontally on smaller devices.

## 2026-07-22 - [Visual Affordances & Semantic Linking]
**Learning:** Found that elements with native hover tooltips (like title tags on badges or shortened dates) lack visual discoverability, and that asynchronous action buttons (like refresh) that dynamically update multiple DOM regions lack semantic connections for screen readers to understand the impact of their action. Additionally, transient actions like 'Copy' buttons aren't marked as disabled while they display their temporary success/error state, which could confuse assistive technologies if the user tries to activate it while locked.
**Action:** Add `cursor: help` and styling like dotted underlines to text with `title` tooltips to visually signal interactivity. Use `aria-controls` on action buttons to semantically link them to the regions they update, and explicitly set `aria-disabled="true"` on transient action buttons during their locked UI state.
