Here are 4 projects perfect for validating backend coding agents.

These projects share a "sweet spot": they have rigid technical constraints (protocols/specs) but open-ended implementation details. This forces the agent to strictly adhere to a standard while managing its own internal architecture.

1. The "Build Your Own Redis" Challenge
   This is arguably the gold standard for testing an agent's ability to handle systems programming, networking, and memory management without getting bogged down in "business logic" ambiguity.

The Core Task: Build a TCP server that listens on a port and implements a subset of the Redis Protocol (RESP).

Why it validates agents:

Parsing Strictness: The agent must implement a custom protocol parser. If it hallucinates the protocol format, the client fails immediately.

Concurrency: It must handle multiple concurrent clients (testing the agent's choice of Goroutines, asyncio, or threading).

State Management: It requires implementing a key-value store with TTL (Time-To-Live) eviction logic.

Validation Steps:

Can it parse a simple PING command?

Can it handle SET key value and GET key?

Can it implement SET key value px 100 (expiry) and correctly return nil after 100ms?

2. A Round-Robin Load Balancer
   This tests an agent's understanding of networking, health checks, and failure recovery.

The Core Task: Create a reverse proxy that accepts incoming HTTP traffic and distributes it across 3–5 mock backend servers.

Why it validates agents:

Error Handling: The critical part is active health checking. If a backend goes down (returns 500 or times out), the balancer must stop sending traffic there. Agents often forget this "negative path" logic.

Algorithm Implementation: It must track state (which server is next?) atomically to avoid race conditions.

Validation Steps:

Start 3 backend servers. Kill one. Does the agent's code still try to route requests there?

Send 10 concurrent requests. Do they get distributed evenly?

3. A Distributed Log (Mini-Kafka)
   A step up in difficulty, focusing on file I/O and data persistence.

The Core Task: Build an HTTP server that accepts data via POST (producer) and allows reading via GET with an offset (consumer).

Why it validates agents:

Persistence: The agent cannot just store data in memory (arrays); it must write to an append-only log file on disk.

Indexing: To read efficiently, the agent should ideally implement a simple index (e.g., "Offset 100 starts at Byte 4024").

Edge Cases: What happens if the user requests an offset that doesn't exist yet?

Validation Steps:

Restart the server. Does the data persist? (This is the #1 failure point for agents—they often default to in-memory storage unless explicitly forced).

4. A Custom SQL-to-JSON Parser
   This tests pure algorithmic logic and recursion, rather than systems knowledge.

The Core Task: Write a function that takes a raw SQL query string (e.g., SELECT name, age FROM users WHERE age > 25) and converts it into a structured JSON Abstract Syntax Tree (AST).

Why it validates agents:

Recursive Logic: Parsing WHERE clauses (especially with nested parentheses like (A OR B) AND C) requires recursive descent or a state machine. LLMs often struggle to keep deep recursion stacks coherent.

Testability: It is purely functional. You can have a test suite of 50 input strings and exactly 50 expected JSON outputs.

Validation Steps:

Feed it a nested query: SELECT \* FROM table WHERE (a=1 OR b=2) AND c=3. Check if the JSON structure correctly groups the OR logic inside the AND logic.
