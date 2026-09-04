## 2025-03-05 - Avoid repeatedly compiling regular expressions
**Learning:** Compiling a regular expression is a relatively expensive operation because it parses the pattern and constructs a state machine. Compiling it inside a function means it gets re-compiled every single time the function is called.
**Action:** Always hoist `regexp.MustCompile` to the package level as a global variable. This ensures it is compiled exactly once when the package is initialized, which is a classic and highly recommended Go performance optimization.

## 2025-03-05 - Avoid multiple string allocations for simple character replacements
**Learning:** Using `strings.ReplaceAll` multiple times or chaining it with `strings.ToUpper` creates intermediate string allocations. Using `regexp.ReplaceAllString` for simple character substitution is also extremely slow compared to raw byte manipulation.
**Action:** For simple string sanitization (like formatting an ID into a valid metric name or environment variable), use a single-pass loop over the string bytes with a pre-sized `strings.Builder` (using `sb.Grow(len(s))`). This eliminates intermediate allocations and is substantially faster.

## 2025-03-05 - Avoid multiple string allocations for simple character replacements
**Learning:** Using `strings.ReplaceAll` multiple times or chaining it with `strings.ToLower` creates intermediate string allocations. Using `regexp.ReplaceAllString` for simple character substitution is also extremely slow compared to raw byte manipulation.
**Action:** For simple string sanitization (like formatting an ID into a valid metric name, environment variable, or pipeline job ID), use a single-pass loop over the string bytes with a pre-sized `strings.Builder` (using `sb.Grow(len(s))`). This eliminates intermediate allocations and is substantially faster.

## 2025-03-05 - Avoid chained string replacements; use fast-path zero-allocation checks
**Learning:** Chaining `strings.ReplaceAll` multiple times creates numerous intermediate string allocations, as each call allocates a brand new string for the entire result even if no changes occur.
**Action:** Always implement a zero-allocation fast-path check first using `strings.IndexByte` to verify if any target characters exist. If none exist, return the original string immediately. If replacements are needed, fall back to a single-pass loop over the string bytes with a pre-allocated `strings.Builder` (`sb.Grow(len(s))`). This guarantees exactly zero or one allocation per string operation, resulting in significant speedups for string formatting utility functions like `sanitizeID` or `safeName`.
## 2024-03-20 - [Redundant checks with strings.ReplaceAll]
**Learning:** `strings.ReplaceAll` internally checks for substring presence before allocating memory.
**Action:** Do not use `strings.Contains` to check if a substring exists before calling `strings.ReplaceAll` in Go, as it results in redundant checks and slower performance.

## 2024-03-21 - [Single pass builder fails with JSON]
**Learning:** When trying to implement a single pass builder using `strings.IndexByte` and `strings.Builder` to prevent multiple string allocations and improve speed, a basic algorithm trying to look for matching `}` after finding a `{` may fail if the string contains a JSON payload. The `{` from the start of the JSON block will be matched with the `}` from the start of the payload block causing everything within to be matched as a key.
**Action:** Always verify if the key matches a value in the `vars` map before blindly writing it out to the builder. If the key does not match any variables in the map, output `{` instead of the original string (since `{` could be the start of a JSON block).

## 2025-05-15 - [Optimize strings.ReplaceAll with strings.NewReplacer]
**Learning:** `strings.NewReplacer` is a standard way to avoid sequential memory allocations and loops compared to chained `strings.ReplaceAll` calls when performing static multi-character replacements. However, its efficiency relies entirely on allocating and building its internal trie structure *once*. If `strings.NewReplacer` is instantiated inline inside a highly-called function, the repeated setup overhead makes it slower than chained `strings.ReplaceAll`.
**Action:** When refactoring chained `strings.ReplaceAll` to `strings.NewReplacer` for performance, always declare the replacer as a package-level global variable (e.g., `var myReplacer = strings.NewReplacer(...)`) so the setup cost is paid only at initialization.

## 2026-03-24 - Critical Path Analysis
**Learning:** Execution time of heavily dependent pipelines is dictated by the critical path of the DAG, not merely the aggregate execution time.
**Action:** Implemented a `--critical-path` command to visualize the longest path utilizing Kahn's topological sort and dynamic programming.

## 2026-03-26 - Single-pass string builder over regex replacement
**Learning:** Using `regexp.ReplaceAllString` combined with `strings.ToLower` creates a huge bottleneck in high-throughput loops due to state machine overhead and multiple allocations. A regex replacement is significantly slower than iterating bytes.
**Action:** For string formatting like sanitizing Kubernetes resource names, drop the regex and use a single-pass `strings.Builder`. This eliminates intermediate allocations and runs nearly 20x faster.

## 2026-03-31 - [Optimize strings.ToLower with strings.EqualFold]
**Learning:** Using `strings.ToLower` for case-insensitive comparisons creates intermediate string allocations. Calling it repeatedly in a loop creates significant overhead.
**Action:** Always use `strings.EqualFold` for case-insensitive string equality comparisons instead of allocating a new string with `strings.ToLower`. `strings.EqualFold` compares the strings in-place without memory allocations, running significantly faster than allocation loops.

## 2025-05-20 - [Zero-allocation String Equality]
**Learning:** Avoid using `strings.ToLower` for case-insensitive string comparisons in hot paths (like `evaluatePendingJobs`), as it allocates a new string.
**Action:** Always use `strings.EqualFold(a, b)` to perform case-insensitive string comparisons without allocating new memory, especially inside loops or frequently called functions.

## 2025-05-20 - [Zero-allocation Substring Matching]
**Learning:** Avoid using `strings.Contains(strings.ToLower(a), b)` in hot loops (like filtering TUI logs or iterating over job lists), as `strings.ToLower` allocates a new string for every check on every loop iteration, creating massive GC pressure.
**Action:** Always implement a custom zero-allocation byte-level check (like `containsFold`) to perform case-insensitive substring matches, which significantly boosts performance by eliminating intermediate string allocations.

## 2024-04-08 - Optimize Job Status Checks with strings.EqualFold
**Learning:** Using `strings.ToLower` on short, frequently evaluated fields like `job.Status` inside loops (e.g., when rendering UI or exporting graphs) allocates intermediate strings that quickly pile up, especially when iterating over many elements.
**Action:** Replace `switch strings.ToLower(s)` with a sequence of `if/else if strings.EqualFold(s, "value")` statements to achieve allocation-free, case-insensitive string matching. This improves performance (e.g., from ~6.2µs to ~1.1µs in a 100-job slice benchmark) and eliminates unnecessary GC pressure.
## 2026-04-12 - Case-Insensitive String Matching Allocations
**Learning:** Using `strings.ToLower` in hot loops (like filtering UI components, parsing logs, or evaluating multiple map keys) creates unnecessary intermediate memory allocations for every checked string. For exact equality, `strings.EqualFold` is zero-allocation. For substring checks, a custom `ContainsFold` function can provide the same benefit without intermediate string creation.
**Action:** Always prefer `strings.EqualFold(a, b)` for exact case-insensitive matches. For case-insensitive substring checks, use the zero-allocation `utils.ContainsFold(s, substr)` rather than converting entire strings to lowercase.

## 2026-05-15 - Case-Insensitive String Checks in File Output Parsing
**Learning:** Checking for substrings or prefixes by converting the entire file output string using `strings.ToLower` creates a huge garbage collection pressure, especially when the file content is large or when it's done frequently.
**Action:** Always use zero-allocation functions like `utils.ContainsFold` and `utils.HasPrefixFold` to do case-insensitive comparisons directly instead of using `strings.ToLower`.
## 2024-05-19 - Allocation-Free String Parsing
**Learning:** Replaced `regexp` usage with native `strings` functions (like `strings.Index`, `strings.TrimSpace`, slicing) for parsing formatted strings (like JSON blocks or custom `<file>` tags). Regular expressions in Go can have high CPU overhead and cause unnecessary heap allocations compared to simple string scanning. In a benchmark, replacing regex in `utils.CleanJSONBlock` gave a ~20x performance improvement (from ~1000ns/op to ~45ns/op).
**Action:** When extracting data from predictable formats (e.g., standard AI output markers), default to `strings.Index` and slicing instead of `regexp.MustCompile`, especially in hot paths or when parsing large texts.
## 2024-05-19 - Allocation-Free String Parsing
**Learning:** Replaced `regexp` usage with native `strings` functions (like `strings.Index`, `strings.TrimSpace`, slicing) for parsing formatted strings (like JSON blocks or custom `<file>` tags). Regular expressions in Go can have high CPU overhead and cause unnecessary heap allocations compared to simple string scanning. In a benchmark, replacing regex in `utils.CleanJSONBlock` gave a ~20x performance improvement (from ~1000ns/op to ~45ns/op).
**Action:** When extracting data from predictable formats (e.g., standard AI output markers), default to `strings.Index` and slicing instead of `regexp.MustCompile`, especially in hot paths or when parsing large texts.

## 2024-05-18 - Case-Insensitive String Checks in Form Submission
**Learning:** Using `strings.ToLower` in hot paths (like checking input form values in `internal/tui/dashboard.go`) creates unnecessary intermediate memory allocations for every checked string.
**Action:** Always prefer `strings.EqualFold(a, b)` for exact case-insensitive matches instead of converting the string with `strings.ToLower`.

## 2024-04-26 - Case-insensitive Keyword Matching Optimization
**Learning:** For case-insensitive keyword searching (e.g. searching for "fix", "password" within strings), `utils.ContainsFold` is vastly faster (up to ~30x-40x) than compiling and executing a case-insensitive regex like `regexp.MustCompile("(?i)(word1|word2)")` because it does zero allocations and avoids regex state machine overhead.
**Action:** Replace `regexp.MustCompile` with chained `utils.ContainsFold` when doing simple case-insensitive substring keyword checks.

## 2025-05-20 - Fast-Path Regex Optimization in Git Output Masking
**Learning:** `maskingWriter.Write` in `internal/git/client.go` was unnecessarily applying `regexp.ReplaceAllString` to every byte slice of Git output to mask GitHub PATs and Basic Auth tokens, causing massive CPU overhead and string allocations, even though >99% of normal git output contains no URLs.
**Action:** When creating I/O stream wrappers or handlers that use regex for sensitive data masking (like secrets or tokens), implement a fast-path zero-allocation check (e.g., `bytes.Contains(p, []byte("https://"))`) first. If the trigger keyword isn't present, return early and bypass all string conversions and regex processing. This provides ~4x performance improvements for raw output streams.

## 2025-05-20 - Avoid string allocations in telemetry log filtering
**Learning:** Using `strings.ToLower(a.Key)` and `strings.Contains()` in a hot path like `slog`'s `ReplaceAttr` (which runs for every attribute of every log message) creates significant unnecessary memory pressure and garbage collection overhead by allocating a new string on each invocation.
**Action:** Always use zero-allocation, case-insensitive substring search methods such as `utils.ContainsFold` to redact sensitive fields or perform string matching in heavily-executed log pipelines.

## 2025-05-21 - Fast-Path Regex Optimization in regex matching
**Learning:** Using `regexp.FindStringSubmatch` unconditionally can be a bottleneck when the target keyword is not present, because of regex state machine overhead.
**Action:** When extracting data using regex that relies on a specific keyword, implement a fast-path zero-allocation check (e.g., `utils.ContainsFold(text, "keyword")`) first. If the trigger keyword isn't present, return early and bypass regex processing.
## 2025-05-21 - [Optimize strings.ToLower with strings.EqualFold]
**Learning:** Using `strings.ToLower` for case-insensitive string comparisons in interactive CLI paths creates unnecessary intermediate memory allocations.
**Action:** Replaced `strings.ToLower(input) == "value"` checks with `strings.EqualFold(input, "value")` to perform case-insensitive string comparisons without allocating new memory, making the code more idiomatic and performant.
## 2025-06-08 - [Optimize substring searching with utils.ContainsFold]
**Learning:** Compiling a regular expression like `regexp.Compile("(?i)" + match)` just to do a case-insensitive substring search in a loop involves unnecessary overhead, and the execution with `MatchString` allocates and does more work.
**Action:** Replace `regexp.Compile("(?i)" + match)` and `matcher.MatchString(str)` with the zero-allocation `utils.ContainsFold(str, match)` when checking for simple substring inclusion.
## 2026-07-04 - [Slice Capacity Allocation in API Filtering]
**Learning:** Pre-allocating slice capacity with `make([]JobInfo, 0, len(jobs))` when filtering large arrays (instead of `var filtered []JobInfo`) significantly reduces memory allocations and improves performance in tight loops, especially where the worst-case size is known.
**Action:** Always use pre-allocated slices for filtered arrays where the upper bound size is equal to the source array.
## 2026-07-06 - Pre-allocating slice capacity in filtering loops\n**Learning:** Pre-allocating slice capacity with `make([]Type, 0, len(source))` when filtering large arrays (instead of `var filtered []Type`) significantly reduces memory allocations and improves performance in tight loops, especially where the worst-case size is known.\n**Action:** Always use pre-allocated slices for filtered arrays where the upper bound size is equal to the source array.
## 2026-07-09 - Pre-allocation conditional hoisting regression
**Learning:** When refactoring Go code to pre-allocate slice capacity for performance (e.g., `make([]Type, 0, len(a)+len(b))`), ensure that expensive data fetching functions or getter methods (like `GetActiveJobs()`) are not inadvertently hoisted out of conditional blocks, as executing them unconditionally introduces severe performance regressions.
**Action:** Always scope slice source data gathering within the conditional block they belong to.
## 2026-07-14 - Pre-allocate Slice Capacity in Filter Iterations over Maps
**Learning:** Initializing zero-capacity slices (`var jobIDs []string`) and subsequently appending to them inside loops that iterate over large internal data structures like `o.activeJobs` and `o.pendingJobs` in the orchestrator causes continuous memory reallocation overhead.
**Action:** When gathering items from maps or slices where the upper bound count is definitively known (e.g., filtering `activeJobs` or `pendingJobs`), always pre-allocate the slice capacity to the total count (`make([]string, 0, len(o.activeJobs)+len(o.pendingJobs))`). This avoids dynamic resizing during append operations and provides a measurable performance boost.
## 2026-07-15 - Pre-allocating slice capacity before appending loops
**Learning:** Found additional cases where zero-value slices (`var slice []Type`) were initialized and subsequently appended to in loops iterating over known sized data structures (`allJobs`, `activeJobs`+`pendingJobs`, `jobs`).
**Action:** Always prefer initializing slices with `make([]Type, 0, exactCapacity)` when iterating over other slices or data structures where the length is determinable, avoiding continuous dynamic memory reallocation overhead.

## 2026-07-20 - Pre-allocating slice capacity before appending loops (Orchestrator APIs & Core loops)
**Learning:** Identified further instances in the orchestrator core logic (e.g. `PurgeJobsByStatus`, job cancellation BFS traversal, and HTTP API filtering in `api.go`) where zero-capacity slices were repeatedly appended to inside loops, causing unnecessary reallocation overhead for commonly fetched list of jobs.
**Action:** When filtering or collecting items from `completedJobs` or `activeJobs`, always pre-allocate the slice using `make([]Type, 0, len(source))` to eliminate dynamic resizing during iterations.
## 2026-07-23 - Lazy evaluation of line number newlines
**Learning:** In Go, when parsing or scanning content (like in `internal/security/scanner.go`), eagerly pre-calculating string offsets (such as finding all newline indices for line number mapping) creates massive CPU overhead and memory allocations even when no matches are found. The typical 'fast path' is zero matches.
**Action:** Defer expensive O(N) pre-calculations until a match is actually found. This lazy evaluation optimizes the common case (no vulnerabilities found) and completely avoids unnecessary string traversals and memory allocations. When you do initialize, pre-allocate slice capacity using `strings.Count`.

## 2025-03-05 - Avoid strings.ToUpper before switch statements
**Learning:** Using `strings.ToUpper(name)` before a switch statement on a string causes an unnecessary heap allocation and a full string traversal. This is especially impactful in hot paths like syntax or text processing.
**Action:** Replace `strings.ToUpper` + `switch` with a zero-allocation `switch len(name)` followed by `strings.EqualFold()` checks inside the cases. This avoids any allocations and performs early returns on length mismatch.
## 2026-08-01 - Avoid strings.ToUpper in parsing Dockerfiles
**Learning:** Using `strings.ToUpper(parts[0])` in `parseDockerfile` creates an unnecessary heap allocation and a full string traversal for every parsed instruction. This is impactful because Dockerfile parsing is a repeated operation during analysis and case-insensitive matching could be done better.
**Action:** Replace `strings.ToUpper` with zero-allocation `strings.EqualFold()` checks inside the rules checks. This avoids all case conversion string allocations during parsing.
## 2026-08-09 - Avoid Redundant Expensive Getter Calls
**Learning:** Found an instance in `internal/orchestrator/api.go`'s `/metrics` handler where `orch.GetActiveJobs()` was called multiple times within a tight block (once in a loop, twice in a subsequent conditional). Because `GetActiveJobs()` acquires a mutex read lock and pre-allocates and populates a new slice on every call, doing this redundantly causes unnecessary lock contention and memory garbage.
**Action:** Always hoist expensive getter methods that acquire locks and allocate slices outside of loops and conditionals when the underlying state is not expected to change during that block, reusing the initial result variable.
## 2026-08-11 - Pre-allocating slice capacity before appending loops
**Learning:** Initializing a zero-capacity slice or pre-allocating a slice with capacity for only a subset of the data (like ) and subsequently appending to it from multiple loops (e.g., iterating over  and then ) causes dynamic memory reallocation overhead if the total appended elements exceed the initial capacity.
**Action:** When gathering items from maps or slices where the upper bound count is definitively known (e.g., filtering  and ), always pre-allocate the slice capacity to the total combined count (e.g., ). This avoids continuous dynamic resizing during append operations and provides a measurable performance boost.

## 2026-08-11 - Pre-allocating full slice capacity for multiple appends
**Learning:** Initializing a zero-capacity slice or pre-allocating a slice with capacity for only a subset of the data (like `make([]string, 0, len(o.activeJobs))`) and subsequently appending to it from multiple loops (e.g., iterating over `activeJobs` and then `pendingJobs`) causes dynamic memory reallocation overhead if the total appended elements exceed the initial capacity.
**Action:** When gathering items from maps or slices where the upper bound count is definitively known (e.g., filtering `activeJobs` and `pendingJobs`), always pre-allocate the slice capacity to the total combined count (e.g., `make([]string, 0, len(o.activeJobs)+len(o.pendingJobs))`). This avoids continuous dynamic resizing during append operations and provides a measurable performance boost.
## 2026-08-14 - Pre-allocating slice capacity before appending loops (API endpoints)
**Learning:** Initializing a zero-capacity slice or pre-allocating a slice with capacity for only a subset of the data (like `make([]JobInfo, 0, len(activeJobs))`) and subsequently appending to it from multiple data sources (e.g., iterating over `activeJobs` and then `completedJobs`) causes dynamic memory reallocation overhead if the total appended elements exceed the initial capacity.
**Action:** When gathering items from multiple slices where the upper bound count is definitively known (e.g., combining `activeJobs` and `completedJobs`), always calculate the total length first and pre-allocate the slice capacity to the total combined count (e.g., `make([]JobInfo, 0, len(activeJobs)+len(completedJobs))`). This avoids continuous dynamic resizing during append operations and provides a measurable performance boost.
## 2026-09-02 - Optimize large log string truncation
**Learning:** In Go, when truncating large strings (such as logs) to keep only the last N lines, using `strings.Split` followed by slicing and `strings.Join` creates excessive memory allocations and overhead, especially for long log outputs.
**Action:** Always optimize performance by avoiding `strings.Split`. Instead, iterate backward through the string to count newlines and extract the substring directly, eliminating intermediate slice allocations.

## 2026-09-03 - Avoid allocating string slices for log truncation
**Learning:** `strings.Split(str, "\n")` allocates an entire slice of strings for the entire file content, even if we only need the last N lines.
**Action:** When truncating large strings (such as logs) to keep only the last N lines, avoid using `strings.Split` followed by slicing and `strings.Join`. Instead, optimize performance by iterating backward through the string to count newlines and extracting the substring directly.
## 2026-09-04 - Optimize string splitting in dashboard log filtering
**Learning:** Using `strings.Split` and `strings.Join` for filtering large log strings creates unnecessary string slice allocations and overhead. The previous optimization to pre-allocate capacity didn't remove the split/join overhead.
**Action:** Avoid `strings.Split` for log filtering. Instead, iterate through the string manually using `strings.IndexByte` to find newlines, check each line, and build the filtered output with `strings.Builder`.
