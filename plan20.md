If there's no way to submit a new job from the Web UI, that is a HUGE missing feature.
Adding a **"Submit New Job" modal** to `webui.go` (and `GET /{$}`) that posts to `POST /jobs`.
The form could have fields for:
- Job ID (optional, auto-generated if blank)
- Summary (required)
- Description (textarea)
- Repo URL (required)
- Priority (number)
- Tags (comma separated)

This would be visually impressive, high value (improves CLI-less usability greatly), and easily testable via Playwright (since we are supposed to use Playwright for frontend verification!).
Wait, memory says:
"To visually verify frontend UI changes in Go-embedded HTML strings (e.g., `internal/orchestrator/webui.go`) via Playwright, extract the HTML subset into a temporary file, load it directly using `page.goto('file://...')`, and capture a screenshot."

This confirms we can and should add frontend features to `webui.go`!
A **"Submit Ad-Hoc Job" Modal/Form** in the Web UI is perfectly aligned. It's high-value, very visible, testable, and satisfies the requirement!
Let's see what `webui.go` looks like right now.
