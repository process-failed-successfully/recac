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

## 2026-07-24 - [Responsive Mobile Layouts for Data Tables]
**Learning:** Found that on narrow viewports, the default body and container padding consumes too much horizontal space, making data-dense tables difficult to read. Furthermore, horizontally aligned header content and action buttons become cramped, resulting in poor touch targets.
**Action:** When implementing responsive mobile layouts (e.g., `@media (max-width: 768px)`), explicitly reduce `body` and container padding to maximize usable width. Additionally, vertically stack header content (`flex-direction: column; align-items: flex-start`) and spread action button groups across the full width (`width: 100%; justify-content: space-between`) to improve touch accessibility and create better touch targets.

## 2026-07-25 - [Text Wrapping on Inline Icon Buttons]
**Learning:** Found that inline action buttons containing icons and text (like a 'Copy ID' button) can awkwardly line-break between the icon and the text when confined within narrow table cells, making the button appear disjointed.
**Action:** When creating inline action buttons (`display: inline-flex`), always add `white-space: nowrap;` to ensure the icon and text remain grouped together as a single visual unit, even when space is constrained.

## 2026-07-26 - [Semantic HTML for Dynamic Timestamps]
**Learning:** Found that using standard non-semantic `<span>` elements for dynamic timestamps (e.g. `Last updated`) loses contextual meaning for assistive technologies, which rely on explicit time data formats (like ISO strings) rather than potentially localized plain text.
**Action:** When inserting timestamps into the DOM, use the semantic `<time>` element and dynamically assign its `datetime` attribute via `toISOString()` to provide a machine-readable, unambiguous date-time format for screen readers and other assistive tools.

## 2024-07-27 - [Priority Badge Visual Affordance]
**Learning:** Using semantic visual shapes (like emojis wrapped in `<span aria-hidden="true">`) alongside color and text for enum-like data badges (such as priority) significantly improves readability and scannability, and helps ensure WCAG 1.4.1 compliance (use of color) for users with color vision deficiencies.
**Action:** Always pair color-coded status text with distinct, semantic visual shapes when building data badges, ensuring they are properly hidden from screen readers to avoid redundant announcements.

## 2026-07-28 - [Transient Button Affordances]
**Learning:** Found that when buttons have a transient locked state applied via `aria-disabled="true"` (like a 'Copied' state), keeping the default `cursor: pointer` or allowing the `:active` scale transform creates false interactivity cues, making users think the locked button can still be clicked.
**Action:** Always explicitly disable interactive cursors (e.g., `cursor: default`) and restrict active state CSS transforms (e.g., `:active:not([aria-disabled="true"])`) on transient UI buttons when they are in their temporary `aria-disabled="true"` state to avoid confusing users.

## 2026-07-29 - [Standardizing Enum Badge Accessibility]
**Learning:** Found that while custom metadata badges (like `Category`) provided proper contextual structure (`title` attribute + `.sr-only` prefix) for screen readers and mouse users, the primary enum-like data badges (Status and Priority) did not. This inconsistency left screen reader users without column context when navigating cells (hearing just "High" instead of "Priority: High"), and lacked visual affordances for native tooltips.
**Action:** When building or standardizing data tables with enum-like inline badges, ensure all badges feature explicit visual affordances (`cursor: help`, `text-decoration: underline dotted`) indicating their interactivity, and always inject visually hidden structural text (e.g., `<span class="sr-only">Priority: </span>`) coupled with a native `title` attribute for comprehensive context.

## 2024-07-30 - Micro-UX improvements for alignment and consistency
**Learning:** Default row header (`th`) styling in data lists can contrast jarringly with standard data cells if not overridden, and flex gap is a more robust way to align icons and text in buttons/badges than margin utilities which can cause awkward wrapping or spacing issues.
**Action:** When styling data tables where the first column is a `th` for accessibility, explicitly set `font-weight: normal` on `tbody th` to match visual expectations. Also, utilize `display: inline-flex; align-items: center; gap: Xpx` along with `white-space: nowrap` for components containing text and icons instead of relying on legacy margins.

## 2026-07-31 - Priority Badge Colors
**Learning:** The priority column previously applied a universal `.status-pending` gray background class to all badges, relying solely on text and semantic emoji icons to distinguish priority levels. This lacked visual hierarchy and made scanning the table difficult.
**Action:** When working with enum-like data badges that indicate severity or urgency (like Priority), ensure that they dynamically apply specific, WCAG AA compliant color classes (e.g., `.priority-high`, `.priority-medium`, `.priority-low`) corresponding to their underlying value, so that visual color cues complement the semantic icons.

## 2024-08-05 - Loading State Animation Feedback
**Learning:** In the Orchestrator web UI, when indicating background data refreshes (e.g., via an `.is-updating` class), static opacity provides insufficient dynamic feedback, making users unsure if the app is still processing. A continuous pulsing animation offers better visual assurance.
**Action:** When implementing updating or loading states, use `@keyframes` for continuous animation (like pulsing) rather than static styling. Crucially, always pair this with a static fallback inside a `@media (prefers-reduced-motion: reduce)` block to respect OS accessibility settings.

## 2026-08-07 - [Keyboard Shortcut Hints]
**Learning:** Found that visual keyboard shortcut hints (like `<kbd>S</kbd>`) were missing from the primary action buttons (like "+ Submit Job") in the Orchestrator web UI, even though the `aria-keyshortcuts` attribute was already present. This made the shortcuts undiscoverable to sighted users. Also found that some empty-state text used `#666` on a light background, which failed WCAG 2.1 AA contrast ratio requirements.
**Action:** Added visual `<kbd>` tags with `aria-hidden="true"` and a custom `.shortcut-hint` CSS class to the buttons, ensuring they are hidden on mobile devices. Updated empty-state text color from `#666` to `#555` to improve accessibility compliance.
## 2026-08-11 - Improve accessibility for timestamps and status badges
**Learning:** In purely JS-driven interfaces, providing visual affordances (like `cursor: help` and dotted underlines) and semantic structural hints (like `<span class="sr-only">`) to enum-like dynamic UI elements significantly improves context for assistive technologies and general usability. Furthermore, timestamps should always use the semantic `<time>` tag with a machine-readable `datetime` attribute rather than raw text.
**Action:** Always append visually hidden structural context (e.g., `<span class="sr-only">Status: </span>`) to dynamic badges, ensure they have proper `title` attributes with matching visual affordances for interactivity, and wrap dynamic times in semantic `<time datetime="...">` tags.
## 2026-08-12 - Transient Action Announcements & Semantic Controls
**Learning:** Extracted inline clipboard handlers into cleaner functions and verified the need to push transient success messages (e.g. "Copied!") using a dedicated `aria-live` announcer instead of changing button labels. Additionally, established that action buttons which update remote DOM regions should explicitly use `aria-controls` to describe those relationships semantically.
**Action:** When adding transient copy functions, set `aria-disabled="true"`, use a dedicated `aria-live` announcer region for feedback, and always bind target update areas to their controlling buttons via `aria-controls`.

## 2026-08-13 - [Responsive Mobile Layout Improvements]
**Learning:** In the Orchestrator web UI, the default body and container padding consumed too much horizontal space on narrow viewports, making data-dense tables difficult to read. Furthermore, horizontally aligned header content and action buttons became cramped, resulting in poor touch targets.
**Action:** When implementing responsive mobile layouts (e.g., `@media (max-width: 768px)`), explicitly reduce `body` and container padding to maximize usable width. Additionally, vertically stack header content (`flex-direction: column; align-items: flex-start`) and spread action button groups across the full width (`width: 100%; justify-content: space-between`) to improve touch accessibility and create better touch targets.

## 2026-08-14 - [Graceful Degradation for Non-JS Users]
**Learning:** In purely JS-driven interfaces like the Orchestrator web UI, users with JavaScript disabled are presented with broken layouts and infinite loading states (e.g., "Connecting..." or "Loading jobs..."). This fails to provide graceful degradation and violates WCAG 1.1.1 fallback content principles.
**Action:** Always implement a `<noscript>` block in JS-driven applications. This block should contain a structured error state explaining the requirement for JavaScript and use a scoped `<style>` to explicitly hide interactive containers and elements with infinite loading states (e.g., setting `display: none !important`).

## 2026-08-16 - [Dynamic Document Title for Background Refreshes]
**Learning:** Found that when the Orchestrator dashboard refreshes in the background, users have no visibility into the active workload unless they switch back to the tab. This degrades the experience of monitoring long-running tasks.
**Action:** Dynamically update the `document.title` during background data refreshes to reflect the active workload (e.g., prefixing with a task count like `(5) Dashboard`) or critical error states, ensuring users have off-page visibility when monitoring background tabs.

## 2026-08-17 - Dynamic Data Table Row Headers
**Learning:** In dynamic data tables, converting the primary column cell (e.g., ID or Name) into a row header (`<th scope="row">`) significantly improves horizontal navigation for screen reader users.
**Action:** Always implement `<th scope="row">` for the primary identifier in data tables, ensuring its CSS styling is adjusted to match standard data cells rather than default headers.

## 2026-08-21 - [Responsive Data Tables]
**Learning:** In the Orchestrator web UI, when implementing responsive data tables using an `overflow-x: auto` wrapper and `table-layout: fixed`, applying a `min-width` (e.g., `600px`) to the `table` element itself is essential. This prevents columns from crushing unreadably on narrow viewports and correctly triggers the wrapper's horizontal scroll.
**Action:** When creating data tables, always apply a `min-width` and `table-layout: fixed` to the `table` element itself to ensure columns don't crush unreadably on narrow viewports and to correctly trigger the wrapper's horizontal scroll.

## 2024-05-23 - Interactive Affordances in Web UI
**Learning:** Native `title` tooltips on text or badges lack obvious visual cues, leading users to miss them. Similarly, buttons without tactile feedback (like an `:active` depressed state) feel less responsive, especially for fast clickers, and missing locked state prevention (`:not([disabled]):not([aria-disabled="true"])`) can lead to confusing false interactivity.
**Action:** Always pair `title` tooltips with clear visual affordances like `cursor: help` and a dotted underline. For buttons, implement a subtle `transform: translateY(1px)` on `:active`, but always ensure it strictly respects both native `disabled` and `aria-disabled="true"` attributes to prevent false tactile feedback on locked elements.
