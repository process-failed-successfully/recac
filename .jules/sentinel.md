## 2025-05-15 - Regex Anchor Bypass
**Vulnerability:** The security scanner's `reRootDeletion` regex used a `$` anchor (`.../$`), causing it to fail if `rm -rf /` was followed by newlines, comments, or other commands.
**Learning:** In Go's `regexp` (RE2), `$` matches the end of the text, not the end of the line (unless `(?m)` is used), and relying on end-of-string anchors for security patterns is brittle against multiline input or appended content.
**Prevention:** Avoid relying on `$` for security pattern matching unless validating the *entire* string structure. Use token boundaries (`\b`, whitespace) or lookahead/separator groups `(?:[\s;]|$)` to define the end of a match context.
