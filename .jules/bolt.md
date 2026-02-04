## BOLT'S JOURNAL

## 2024-05-23 - [Stream Log Parsing]
**Learning:** Reading entire files into memory with `os.ReadFile` then splitting strings causes massive memory pressure. `bufio.Scanner` is much better but requires `scanner.Buffer` adjustment for large lines (default 64KB).
**Action:** Always prefer streaming `io.Reader` for potentially large inputs.
