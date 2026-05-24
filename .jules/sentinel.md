## 2024-05-24 - Enable CLI bulk priority update
**Vulnerability:**
None (Enhancement).

**Learning:**
The updateBulkPriority function existed but was never invoked in main.go, rendering the --update-priority-tag, --update-priority-match, and --update-priority-group CLI flags non-functional despite being parsed and validated. Added the necessary dispatch logic in main.go.

**Prevention:**
Ensure CLI features are wired end-to-end and test all command branches, not just single-item actions.
