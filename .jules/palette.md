## 2024-03-24 - Accurate Keyboard Hinting in Bubble Tea TUIs
**Learning:** Keyboard instructions must accurately reflect all supported keybindings in the Bubble Tea `Update` function to prevent user confusion and improve accessibility, particularly when multiple keys (like Space and Enter) perform the same action.
**Action:** Always verify the actual key handling logic (e.g., `if msg.String() == " " || msg.String() == "enter"`) in the component's `Update` loop before setting the instructional text in the `View` function.
## 2024-05-15 - Identify accurate catch-all states in Update Loops
**Learning:** Sometimes keyboard instructions in the UI specify a single key (like "Press q to quit") when the `Update` loop actually uses a catch-all for the current state (e.g. `if state == Finished { return Quit }` outside the `switch msg.String()`). This creates a discrepancy where *any* key performs the action but the UI doesn't say so.
**Action:** Always check if a state condition in the `tea.KeyMsg` case catches all inputs regardless of the specific key string. Update the UI hint (e.g., "Press any key to quit") to match the catch-all logic and prevent users from feeling restricted.

## 2026-03-05 - Reciprocal Actions in Empty States
**Learning:** When users toggle a view (e.g., showing history) and arrive at an empty state, the UI must provide a clear call-to-action to return to the previous state to avoid getting stuck.
**Action:** Always include the reverse keybinding hint (like "Press 'h' to return") in empty state messages for toggled views.
