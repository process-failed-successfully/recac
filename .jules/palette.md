## 2025-05-15 - Confirming Destructive Actions in TUI
**Learning:** TUI users expect keyboard-driven workflows to be fast, but destructive actions (like killing a session) without confirmation lead to anxiety and potential data loss. Adding a simple modal state dramatically improves confidence.
**Action:** Always wrap destructive actions (delete, kill, stop) in TUIs with a distinct 'confirmation mode' that requires explicit user intent (e.g., pressing 'y').
