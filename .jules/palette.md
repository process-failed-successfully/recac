## 2024-03-24 - Accurate Keyboard Hinting in Bubble Tea TUIs
**Learning:** Keyboard instructions must accurately reflect all supported keybindings in the Bubble Tea `Update` function to prevent user confusion and improve accessibility, particularly when multiple keys (like Space and Enter) perform the same action.
**Action:** Always verify the actual key handling logic (e.g., `if msg.String() == " " || msg.String() == "enter"`) in the component's `Update` loop before setting the instructional text in the `View` function.
## 2024-05-15 - Identify accurate catch-all states in Update Loops
**Learning:** Sometimes keyboard instructions in the UI specify a single key (like "Press q to quit") when the `Update` loop actually uses a catch-all for the current state (e.g. `if state == Finished { return Quit }` outside the `switch msg.String()`). This creates a discrepancy where *any* key performs the action but the UI doesn't say so.
**Action:** Always check if a state condition in the `tea.KeyMsg` case catches all inputs regardless of the specific key string. Update the UI hint (e.g., "Press any key to quit") to match the catch-all logic and prevent users from feeling restricted.

## 2026-03-05 - Reciprocal Actions in Empty States
**Learning:** When users toggle a view (e.g., showing history) and arrive at an empty state, the UI must provide a clear call-to-action to return to the previous state to avoid getting stuck.
**Action:** Always include the reverse keybinding hint (like "Press 'h' to return") in empty state messages for toggled views.

## 2026-03-05 - Actionable Empty States
**Learning:** Empty states should not just tell the user what's missing, but proactively guide them on what to do next. When a list is empty, including a primary action (like "Submit a new job" in the Orchestrator dashboard) makes the UI significantly more actionable and intuitive.
**Action:** Always include a helpful call-to-action in empty state messages, suggesting the most logical next step for the user and providing the relevant keybinding.

## 2026-03-05 - Overlay Actionable Empty States
**Learning:** When displaying actionable empty states in overlay views (like `viewTree` in the Orchestrator dashboard) where primary actions (like 's' to submit) are only handled by the main view, the empty state instruction must explicitly guide the user to return to the main view first.
**Action:** Use phrasing like "Press 'q' or 'esc' to go back, then press 's' to submit a new job" rather than just "Press 's' to submit", which would falsely imply the key works directly in the overlay.
## Palette's Journal
## 2024-03-14 - Modal Close Buttons Accessibility
**Learning:** In the orchestrator dashboard, modal close buttons were previously implemented as `<span class="close">` elements, which lack native focusability, keyboard event handling (Space/Enter), and semantic meaning for screen readers. Using `span` for interactive elements causes severe accessibility barriers.
**Action:** Always replace interactive `<span onclick="...">` elements with `<button type="button">` and apply an `aria-label` (e.g., `aria-label="Close modal"`) when the content is purely visual (like `&times;`). Additionally, ensure a `:focus-visible` CSS rule is added for clear keyboard navigation cues.
## 2026-03-17 - Contextual aria-labels in data tables
**Learning:** When generating repeating rows of data with action buttons, screen readers will announce every button as just its text content (e.g. "Cancel"). This forces visually impaired users to guess which row the button belongs to.
**Action:** Always include contextual `aria-label` attributes on inline action buttons in data tables (e.g. `aria-label="Cancel job {id}"`) to explicitly link the action to the item.

## 2026-03-19 - Added focus-visible states to form controls in Web UI
**Learning:** The embedded HTML web UI dashboard in `internal/orchestrator/webui.go` lacked proper focus indicators for standard form inputs (`<input>`, `<textarea>`, `<select>`), despite having them for buttons. This is a common oversight in custom dashboards that break standard keyboard navigation accessibility.
**Action:** Applied `:focus-visible` to ensure outline visibility for keyboard users. Need to verify focus states for all interactive elements in embedded web UIs, not just primary buttons.

## 2026-03-24 - Semantic Form Wrappers for Custom Modals
**Learning:** Custom UI components (like Modals) often use standard `<input>` elements but rely on JS-bound click events (e.g., `onclick` on a standard button) to submit, ignoring semantic `<form>` submission. This breaks native "Enter-to-submit" accessibility for keyboard users and forces developers to use jarring `alert()` or manual empty-state validation in JS.
**Action:** Always wrap actionable inputs inside custom modals with a `<form onsubmit="mySubmitFunc(); return false;">` tag. Change the action button to `type="submit"` and use the HTML5 `required` attribute on inputs. This naturally guides screen readers, enables Enter-to-submit out of the box, and provides native tooltips for validation rather than blocking JS alerts.

## 2026-03-22 - Modal Dismissal Accessibility in Embedded Dashboards
**Learning:** Custom vanilla JS modals in embedded dashboards (like `internal/orchestrator/webui.go`) often lack basic dismissal accessibility, forcing users to click a specific "x" button. This is frustrating for keyboard users and those accustomed to modern web patterns.
**Action:** Always implement global event listeners for the `Escape` key (`keydown`) and outside clicks (`window.click`) to close open modals. When doing so, ensure that any active state tied to the modal (like an abortable log stream) is also properly cleaned up during the closure.
## 2025-06-20 - [Semantic Forms in Modals]
**Learning:** Implementing `<form>` tags with HTML5 validation (like `required`) inside custom JS modals not only improves screen reader accessibility by defining form boundaries but also replaces clunky manual JS `alert()` validation with native, localized tooltips.
**Action:** Always wrap actionable inputs and submit buttons in semantic `<form>` tags with `onsubmit` handlers, rather than relying purely on `<button type="button" onclick="...">`.

## 2024-05-19 - Screen Reader Modals
**Learning:** Vanilla JS modals defined with just CSS classes (`.modal`) in pure HTML strings aren't inherently recognized as dialogs by screen readers, leading to confusing navigation states for users relying on assistive tech.
**Action:** Always include `role="dialog"` and `aria-modal="true"` directly on the parent modal wrapper element when building manual HTML modals to ensure they trap screen reader context properly.
## 2024-03-27 - Added Loading States to Async Buttons

**Learning:** Buttons triggering async network requests (like `fetch`) that don't have loading states can be frustrating to users because they lack visual feedback, making users unsure if their click registered, potentially leading to double-submissions.
**Action:** Always ensure async buttons disable themselves and update their text (e.g., to "Wait..." or a spinner) while the request is in flight, and use a `finally` block to guarantee the original state is restored.

## 2026-03-30 - Sanitize CSS Class Names Built from Data
**Learning:** Job statuses in the Orchestrator WebUI can contain spaces (e.g., "Pending Approval"). When dynamically injecting these into HTML as CSS class names (like `class="status-${status}"`), spaces create invalid multiple class assignments, breaking styles.
**Action:** Always sanitize dynamic strings replacing spaces with hyphens (e.g., `status.replace(/\s+/g, '-')`) before using them as class names, and ensure matching CSS rules exist for all possible states (like `.status-Canceled`, `.status-Error`, `.status-Pending-Approval`).

## 2026-03-31 - Focus Visible on Checkbox and Number Inputs
**Learning:** In custom dashboards, form-group level CSS targeting `:focus-visible` (e.g. `.form-group input[type="text"]:focus-visible`) often inadvertently omits other standard input types like `number` or `checkbox`, leaving them completely inaccessible to keyboard users because they lack a focus ring.
**Action:** Always ensure that `input[type="number"]` and `input[type="checkbox"]` have explicitly defined `:focus-visible` outline styles, alongside standard `text` and `textarea` inputs, to guarantee comprehensive keyboard navigation accessibility.

## 2026-04-05 - Global Keyboard Shortcuts in Custom Web Dashboards
**Learning:** Adding global keyboard shortcuts (like `/` to focus search) to embedded web UIs is a huge usability win, but it's crucial to explicitly prevent them from triggering while typing in inputs (`event.target.tagName`) or when any modal is open (`.modal` check). If ignored, users might accidentally refresh data or steal focus while simply trying to type characters or navigate an overlay.
**Action:** Always wrap global document-level `keydown` event listeners with checks for active form elements (`INPUT`, `TEXTAREA`, `SELECT`) and open modals before executing the shortcut logic.

## 2026-04-06 - Announce Dynamic Updates to Screen Readers
**Learning:** In dynamically updated UI components (like viewing logs, analyzing failures, or explaining job details in a modal), screen readers will not naturally announce text content that is injected asynchronously after the container is already rendered.
**Action:** Always add `aria-live="polite"` to the container element (e.g. `<div id="analyze-failures-content" aria-live="polite">`) where dynamic text updates will occur, so that screen readers correctly notify visually impaired users without interrupting their current tasks.
## 2024-04-05 - Missing aria-live in Async Modals
**Learning:** Asynchronous content updates in modals (like graphs, timelines, and dry-run results) and main dashboard dynamic content (like status and analytics) were not being announced to screen readers.
**Action:** Applied `aria-live="polite"` to the dynamic container elements so updates are smoothly read out after network fetches or SSE updates complete.
## 2026-04-08 - TUI Keybinding Hint Accuracy\n**Learning:** When adding keyboard instructions to Bubble Tea TUI components, failing to verify the actual key handling logic in the `Update` loop can result in incomplete hints (e.g., showing only 'tab' when 'up' and 'down' are also supported), confusing users.\n**Action:** Always cross-reference the UI hint in the `View` logic with the actual handled inputs in the component's `Update` loop to ensure accuracy.

## 2026-04-10 - Screen Reader Redundancy on Required Fields
**Learning:** Relying solely on the HTML5 `required` attribute combined with a visual asterisk (e.g. `*`) inside a `<label>` can lead to screen readers inconsistently announcing the required state or redundantly announcing "star".
**Action:** Always complement the `required` attribute with `aria-required="true"` on the input element for robust screen reader support. Additionally, wrap the visual asterisk in the label with `<span aria-hidden="true">` to prevent screen readers from reading it out loud.

## 2026-04-13 - Actionable Keybindings in Modals
**Learning:** When triggering a modal or input form via a global keyboard shortcut in a web UI, ensure immediate accessibility by automatically focusing the primary input field using `setTimeout(() => element.focus(), 10)` to prevent the user from needing to manually click.
**Action:** Always auto-focus primary inputs when opening modals via shortcuts.

## 2026-04-14 - Update WebUI tests when modifying WebUI HTML
**Learning:** When modifying HTML or JavaScript functions in `internal/orchestrator/webui.go` (such as adding parameters to functions or altering HTML attributes), always ensure to update the corresponding string matching assertions in the UI tests, specifically within `internal/orchestrator/api_webui_test.go`.
**Action:** Run `go test ./internal/orchestrator/ -run TestAPI_WebUI_Actions -v` to check.

## 2026-04-15 - Data Table Row Hover States
**Learning:** Data-dense tables in dashboards (like the Jobs table) without row hover states make it difficult for users to track data across a single row, increasing cognitive load.
**Action:** Always add a subtle `tbody tr:hover` background color to data tables to improve scannability and provide visual feedback during interaction.

## 2026-04-18 - Actionable Empty States
**Learning:** Unstyled or unhelpful empty states in modals (like "No data found") leave users stuck without guidance on how to proceed.
**Action:** Always style empty states centrally with clear, actionable keybinding instructions (e.g., "Press 'Esc' to go back") to improve UX.

## 2026-04-20 - Extending Auto-Focus and Native Keyboard Shortcut Exposing
**Learning:** We previously learned to auto-focus primary inputs when opening modals via keyboard shortcuts. However, click-triggered modals suffer the same accessibility issue—forcing a mouse user to click again to start typing. Furthermore, global keyboard shortcuts defined via JS `keydown` listeners are invisible to screen readers unless explicitly marked.
**Action:** Always add `setTimeout(() => element.focus(), 10)` to the `onclick` handlers or JS functions for click-triggered modals to immediately focus the primary input field. Also, use the `aria-keyshortcuts` attribute on elements that have JS-bound keyboard shortcuts so assistive technologies can announce them natively.

## 2026-04-21 - Accessible Text Colors for Status Indicators
**Learning:** Native named web colors like 'red' (`#FF0000`) and 'orange' (`#FFA500`) often fail WCAG AA text contrast guidelines against light backgrounds (e.g. `#f4f4f4` and `#ffffff`), causing readability issues for visually impaired users.
**Action:** Always avoid native named colors for text statuses and instead use accessible hex codes that meet the 4.5:1 contrast ratio, such as Bootstrap's `#d32f2f` for danger/error, `#198754` for success, and `#b45309` for warning states.
## 2026-04-21 - Accessible Text Colors for Status Indicators
**Learning:** Native named web colors like 'red' (`#FF0000`) and 'orange' (`#FFA500`) often fail WCAG AA text contrast guidelines against light backgrounds (e.g. `#f4f4f4` and `#ffffff`), causing readability issues for visually impaired users.
**Action:** Always avoid native named colors for text statuses and instead use accessible hex codes that meet the 4.5:1 contrast ratio, such as Bootstrap's `#d32f2f` for danger/error, `#198754` for success, and `#b45309` for warning states.

## 2026-04-22 - Empty States for AI Generated Content
**Learning:** AI-generated string outputs (like changelogs, postmortems, or explanations) often just return an empty string when no data is available. Rendering this as a bare "No explanation provided" text node within a modal is visually inconsistent and lacks guidance compared to structural data empty states.
**Action:** Always wrap text-based empty states in the same actionable empty state styling as structured data (e.g., using a centered div with clear keybinding instructions like "Press 'Esc' to close").

## 2026-04-23 - Focusable Scrollable Containers
**Learning:** Native scrollable containers (`overflow: auto` or `overflow-y: auto`) without inherently focusable elements inside them are completely inaccessible to keyboard-only users, preventing them from scrolling through content like logs, long graphs, or analysis reports.
**Action:** Always add `tabindex="0"` to containers with `overflow` properties (along with a `:focus-visible` outline) to ensure they can receive focus and be scrolled using the keyboard arrow keys.

## 2026-05-01 - Empty States for Generated Content Should Include Close Instruction
**Learning:** We previously learned that AI-generated string outputs should be rendered using the same styled, actionable empty state pattern as structured data. I noticed that we had missed a few empty states for "No flaky jobs found" and "No failing jobs found" in the reliability modal analysis, which were just bare `<p>` tags without an explicit close instruction.
**Action:** Ensure all empty states within modals have consistent styling (`text-align: center; padding: 2em; color: #666;`) and provide an explicit instruction like "Press 'Esc' to close." so users know how to proceed.
## 2026-04-29 - Explicit Form Labels vs Placeholders
**Learning:** Relying solely on 'placeholder' and 'aria-label' attributes for form inputs (like in searchLogsModal and editDepsModal) causes usability issues for visual users because the context disappears as soon as they start typing. The aria-label is inaccessible to visual users.
**Action:** Always pair inputs with explicit, visible <label> elements, even in dense horizontal flex layouts (where 'align-items: flex-end' can be used to neatly align the labels above the inputs and buttons).
## 2026-04-30 - Explicit Form Labels vs Placeholders
**Learning:** Relying solely on 'placeholder' and 'aria-label' attributes for form inputs (like in searchLogsModal and editDepsModal) causes usability issues for visual users because the context disappears as soon as they start typing. The aria-label is inaccessible to visual users.
**Action:** Always pair inputs with explicit, visible <label> elements, even in dense horizontal flex layouts (where 'align-items: flex-end' can be used to neatly align the labels above the inputs and buttons).

## 2026-05-01 - Confirmation for Destructive Row Actions
**Learning:** Tightly packed data tables with inline action buttons (like Cancel, Purge, Skip) are highly susceptible to accidental clicks. If these buttons map directly to destructive API calls without a client-side confirmation, users can easily cause unrecoverable data loss or interrupt critical flows.
**Action:** Always wrap inline destructive actions in data tables with a `confirm()` dialog to provide a friction layer against accidental clicks.
