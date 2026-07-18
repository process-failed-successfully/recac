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
