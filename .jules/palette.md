## 2025-05-15 - Confirming Destructive Actions in TUI
**Learning:** TUI users expect keyboard-driven workflows to be fast, but destructive actions (like killing a session) without confirmation lead to anxiety and potential data loss. Adding a simple modal state dramatically improves confidence.
**Action:** Always wrap destructive actions (delete, kill, stop) in TUIs with a distinct 'confirmation mode' that requires explicit user intent (e.g., pressing 'y').

## 2025-05-15 - Explicit Selection Indicators in TUI Lists
**Learning:** In TUI lists, the cursor position is often confused with the 'active' state. When a list is used for selection (like a settings menu), users need to see which item is *currently* active, separate from which item they are *hovering* over.
**Action:** When rendering selection lists, always append a visual marker (like " (Current)" or a checkmark icon) to the item that represents the current state.
