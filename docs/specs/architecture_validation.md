# Hallucination-Proof Architecture: The "Ironclad Contract"

## Objective
To ensure that all code generation tasks are based on a mathematically valid, consistent, and hallucination-free architectural plan *before* any Jira tickets are created or agents are spawned.

## The Core Concept: Schema-First Validation
Instead of generating tickets directly from natural language (which is prone to hallucinations and mismatched interfaces), we introduce an intermediate **Strict Schema Layer**.

### The New Flow
1.  **Input:** `app_spec.txt` (Natural Language Goals)
2.  **Architect Agent:** Decomposes the spec into a machine-readable `architecture.yaml` + `contracts/` (OpenAPI/Go Interfaces).
3.  **The Validator (Go):** A deterministic compiler that verifies the architecture graph.
    *   *Check:* Do all referenced APIs exist?
    *   *Check:* Do Component A's outputs match Component B's inputs?
    *   *Check:* Are there cycles or orphans?
4.  **Ticket Generator:** Deterministically converts the *validated* `architecture.yaml` into Jira Epics/Stories.

---

## 1. The Architecture Schema (`architecture.yaml`)

This file defines the "Truth" of the system.

```yaml
version: "1.0"
system_name: "payment-processor"

# Global Data Definitions (can be referenced)
types:
  - name: "User"
    schema: "./schemas/user.json" # or embedded

# The Building Blocks
components:
  - id: "api-gateway"
    type: "service"
    description: "Public facing API"
    contracts:
      - type: "openapi"
        path: "./contracts/openapi_v1.yaml"
    consumes: []
    produces:
      - event: "HTTPRequest"
        target: "payment-service"

  - id: "payment-service"
    type: "worker"
    description: "Processes payments"
    inputs:
      - source: "api-gateway"
        type: "HTTPRequest"
        schema: "./schemas/http_req.json"
    outputs:
      - type: "PaymentProcessedEvent"
        schema: "./schemas/events/payment_processed.json"

  - id: "audit-log"
    type: "database"
    description: "Postgres DB for audit"
    inputs:
      - source: "payment-service"
        type: "PaymentProcessedEvent"

# Validation Rules (Explicit)
flow_constraints:
  - "payment-service MUST output PaymentProcessedEvent"
```

## 2. The Architect Agent
**Role:** The only "creative" AI in this phase.
**Input:** `app_spec.txt`
**Task:**
1.  Design the system components.
2.  **Write the Contracts first.** (Generate the OpenAPI spec, the SQL schema, the JSON event schemas).
3.  Output the `architecture.yaml` linking these artifacts.

*The Agent is not allowed to write implementation code, only Interfaces/Schemas.*

## 3. The Validator (Go Tool)
A strict static analysis tool run locally.

```go
func ValidateArchitecture(arch SystemArchitecture) error {
    // 1. Load all contracts
    // 2. Verify topological sort (no cycles if DAG required)
    // 3. Type Check:
    for _, comp := range arch.Components {
        for _, out := range comp.Outputs {
            // Find who consumes this
            consumer := FindConsumer(out.Type)
            if consumer == nil {
               return fmt.Errorf("Output %s from %s goes nowhere", out.Type, comp.ID)
            }
            // Verify Schema Compatibility
            if !SchemasMatch(out.Schema, consumer.InputSchema) {
                return fmt.Errorf("Type Mismatch: %s produces X but %s expects Y", comp.ID, consumer.ID)
            }
        }
    }
    return nil
}
```

## 4. Deterministic Ticket Generation
Once `architecture.yaml` passes validation, we generate tickets. These tickets are **not** prompt-based hallucinations. They are templated tasks.

**Example Mapping:**

| Architecture Element | Jira Ticket Type | Ticket Title | Ticket Body |
| :--- | :--- | :--- | :--- |
| `component: payment-service` | **Epic** | `[Service] Payment Service` | "Implement the service matching spec `contracts/openapi_v1.yaml`" |
| `input: HTTPRequest` | **Story** | `[Impl] Handle HTTPRequest` | "Implement handler for `HTTPRequest`. Input Schema: `schemas/http_req.json`." |
| `output: PaymentProcessed` | **Story** | `[Impl] Emit PaymentProcessed` | "Publish event. Schema: `schemas/events/payment_processed.json`." |

## Benefits
1.  **Hallucination Proof:** If the architect hallucinates a non-existent input, the **Validator** catches it immediately ("Component B expects X, but nobody produces X").
2.  **Parallelism:** Since contracts are generated *and committed* in Step 2, implementation agents can start on "Consumer" and "Producer" simultaneously without talking to each other.
3.  **Testability:** The generated contracts *become* the test fixtures.
