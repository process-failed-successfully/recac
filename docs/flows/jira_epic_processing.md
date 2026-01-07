# Jira Epic Processing Flow

This diagram illustrates the detailed flow of how `recac` processes a Jira Epic containing multiple tickets.
The system relies on Jira Issue Links (specifically "blocks" / "is blocked by") to handle dependencies between tickets.

## Key Concepts

1.  **Epic & Tickets**: To `recac`, an Epic and its Child Tickets are all just "Issues" to be polled.
2.  **Dependencies**: Order is enforced via "Blocks" links. If Ticket A blocks Ticket B, `recac` will not process B until A is "Done".
3.  **Polling**: The Orchestrator periodically polls Jira for all relevant tickets.
4.  **External Blockers**: A ticket is only "Ready" if ALL its blockers (both those in the current batch and those outside it) are "Done".

```mermaid
sequenceDiagram
    participant User
    participant Jira
    participant Orchestrator
    participant DependencyGraph
    participant Spawner
    participant Agent(Job)

    Note over User, Jira: **Setup Phase**
    User->>Jira: Create Epic "E-1"
    User->>Jira: Create Ticket "T-1" (Child of E-1)
    User->>Jira: Create Ticket "T-2" (Child of E-1)
    User->>Jira: Link: T-1 "blocks" T-2
    User->>Jira: Link: T-2 "blocks" E-1 (Optional, enforces Epic waits for tickets)
    User->>Jira: Add Label "recac-agent" to T-1, T-2, E-1

    Note over Orchestrator, Jira: **Polling Cycle 1**
    loop Every 1 Minute
        Orchestrator->>Jira: Search Issues (JQL: labels="recac-agent" AND status!=Done)
        Jira-->>Orchestrator: Returns [T-1, T-2, E-1]
        
        Orchestrator->>DependencyGraph: Build Graph([T-1, T-2, E-1])
        Note right of DependencyGraph: Internal Dependencies:<br/>T-1 -> T-2 -> E-1
        DependencyGraph-->>Orchestrator: Ready Candidates (Roots): [T-1]
        
        Orchestrator->>Jira: GetBlockers(T-1)
        Jira-->>Orchestrator: [] (No external blockers)
        
        Orchestrator->>Orchestrator: Identify WorkItem: T-1
        
        Orchestrator->>Jira: Transition T-1 to "In Progress"
        Orchestrator->>Spawner: Spawn Agent for T-1
        Spawner->>Agent(Job): Start Container (Repo, Task)
    end

    Note over Agent(Job): **Execution Phase (T-1)**
    Agent(Job)->>Agent(Job): Clone Repo
    Agent(Job)->>Agent(Job): Execute Task (Plan -> Code -> Verify)
    
    alt Task Success
        Agent(Job)->>Jira: Add Comment "Task Complete..."
        Agent(Job)->>Jira: Transition T-1 to "Done"
    else Task Failure
        Agent(Job)->>Jira: Add Comment "Failed: error..."
        Agent(Job)->>Jira: Transition T-1 to "Failed" (or similar)
    end
    
    Note over Orchestrator, Jira: **Polling Cycle 2 (After T-1 is Done)**
    loop Next Minute
        Orchestrator->>Jira: Search Issues (JQL: labels="recac-agent" AND status!=Done)
        Jira-->>Orchestrator: Returns [T-2, E-1] (T-1 is Done, excluded)
        
        Orchestrator->>DependencyGraph: Build Graph([T-2, E-1])
        Note right of DependencyGraph: Internal Dependencies:<br/>T-2 -> E-1
        DependencyGraph-->>Orchestrator: Ready Candidates: [T-2]
        
        Orchestrator->>Jira: GetBlockers(T-2)
        Jira-->>Orchestrator: Returns [T-1 (Done)] -> Filtered out
        Note right of Orchestrator: T-1 is Done, so T-2 is NOT blocked.
        
        Orchestrator->>Orchestrator: Identify WorkItem: T-2
        
        Orchestrator->>Jira: Transition T-2 to "In Progress"
        Orchestrator->>Spawner: Spawn Agent for T-2
    end

    Note over Agent(Job): **Execution Phase (T-2)**
    Agent(Job)->>Agent(Job): Execute T-2...
    Agent(Job)->>Jira: Transition T-2 to "Done"

    Note over Orchestrator, Jira: **Polling Cycle 3 (After T-2 is Done)**
    loop Next Minute
        Orchestrator->>Jira: Search Issues
        Jira-->>Orchestrator: Returns [E-1]
        
        Orchestrator->>DependencyGraph: Build Graph([E-1])
        DependencyGraph-->>Orchestrator: Ready Candidates: [E-1]
        
        Orchestrator->>Jira: GetBlockers(E-1)
        Jira-->>Orchestrator: Returns [T-2 (Done)] -> Filtered out
        
        Orchestrator->>Orchestrator: Identify WorkItem: E-1
        Orchestrator->>Jira: Transition E-1 to "In Progress"
        Orchestrator->>Spawner: Spawn Agent for E-1 (Final Review/Merge?)
    end
```
