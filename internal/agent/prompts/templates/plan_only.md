# Implementation Plan

You are a Senior Software Architect. Your task is to analyze the provided application specification and codebase context to generate a detailed, step-by-step implementation plan.

**Goal:** Create a plan that a Junior Developer or an AI Agent can follow to implement the requested features.

## Input Context

### Application Specification
```
{spec}
```

### Codebase Context
```
{codebase_context}
```

## Instructions

1.  **Analyze**: Understand the requirements and the existing code structure.
2.  **Break Down**: Decompose the task into small, logical steps.
3.  **Detail**: For each step, describe:
    -   **What** needs to be done.
    -   **Which files** need to be created or modified.
    -   **Key logic** or algorithms involved.
    -   **Verification**: How to verify this step is working (e.g., "Run test X").
4.  **Order**: Ensure steps are in a logical dependency order.

## Output Format

Return the plan in Markdown format.

```markdown
# Implementation Plan: [Project Name]

## Overview
[Brief summary of the approach]

## Phased Plan

### Phase 1: [Phase Name]
- [ ] **Step 1**: [Description]
  - *Files*: `path/to/file.go`
  - *Details*: [Implementation details]
  - *Verification*: [Test command or check]

- [ ] **Step 2**: ...

### Phase 2: ...
```

Do not write any code. Focus on the *plan*.
