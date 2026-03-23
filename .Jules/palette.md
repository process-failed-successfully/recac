## 2025-06-20 - [Semantic Forms in Modals]
**Learning:** Implementing `<form>` tags with HTML5 validation (like `required`) inside custom JS modals not only improves screen reader accessibility by defining form boundaries but also replaces clunky manual JS `alert()` validation with native, localized tooltips.
**Action:** Always wrap actionable inputs and submit buttons in semantic `<form>` tags with `onsubmit` handlers, rather than relying purely on `<button type="button" onclick="...">`.
