## 2025-05-15 - [CPD Performance: SHA256 vs FNV1a]
**Learning:** For sliding window hashing of hash values, using a cryptographic hash like SHA256 is overkill and 100x slower than a simple FNV-1a style mixing loop. Since the input values are already hashes (high entropy), simple XOR mixing is effective and sufficient for collision resistance in this context.
**Action:** When hashing internal data structures for map keys (not security), prefer simple mixing functions (FNV, Murmur, or even just XOR if entropy is high) over cryptographic hashes.
