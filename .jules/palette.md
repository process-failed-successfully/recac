## 2026-07-13 - [Fix UI Crash & Enhance Action Affordances]
**Learning:** Found that `replace` string operations on Go numeric variables (`int` passed to JS) crash DOM rendering (causing the features table to vanish and show a fake network error). Also learned that embedded "Retry" and "Refresh" action buttons in empty states were difficult to scan quickly.
**Action:** Always wrap integer-based properties in `String()` before performing string methods like `.replace()`. Also, prepended visually distinct icons (`<span aria-hidden="true">🔄</span> `) to repetitive interactive table/graph recovery actions for better scannability.

## 2026-07-14 - [Graceful Degradation for JS-Driven Interfaces]
**Learning:** Found that purely client-side rendered dashboards trapped non-JS users (or users on failing networks) in an infinite "Loading..." state because the DOM elements containing spinners are baked into the static HTML, relying entirely on JS to clear them.
**Action:** Implemented a `<noscript>` block inside the main layout containing a scoped `<style>` that sets `display: none !important` on interactive containers and shows a structured error state. This ensures a graceful, accessible fallback (WCAG 1.1.1 fallback content) rather than a broken UI.
