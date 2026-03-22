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
