# Rewrite of Combined Autonomous Coding (recac) - TODO

Agent expansion:
QA gets the specific commit where a feature is completed and tests and validates the change.
    Continuous validation and updates
Scalability Agent
Security Agent
Split manager and Techlead roles
    Manager - Ensures no scope creep and project matches app spec
    Tech lead - Validates approach and core logic
Multiagent - File locking to avoid collission/clobber

The feature list should be made more granular and then consumed into the DB as a queue.

That way smaller cheaper agents and even large ones can pull from the queue, or get passed work from the orchestrator.