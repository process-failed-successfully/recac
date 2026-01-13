## 2025-05-20 - Contextual Help in TUI
**Learning:** Users often get stuck in TUI submenus (like lists) because navigation keys (Up/Down/Esc) aren't explicitly shown in the footer help, which defaults to global keys.
**Action:** Implement a dynamic `keyMap` wrapper that adjusts the `ShortHelp` output based on the active UI mode (e.g., showing "select/back" in menus vs "send" in chat).
