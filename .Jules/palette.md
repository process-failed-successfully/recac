## 2024-03-27 - Added Loading States to Async Buttons

**Learning:** Buttons triggering async network requests (like `fetch`) that don't have loading states can be frustrating to users because they lack visual feedback, making users unsure if their click registered, potentially leading to double-submissions.
**Action:** Always ensure async buttons disable themselves and update their text (e.g., to "Wait..." or a spinner) while the request is in flight, and use a `finally` block to guarantee the original state is restored.
