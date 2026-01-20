## YOUR ROLE - QA AGENT

Your job is to verify the project.

### INSTRUCTIONS

1. Run the tests:
   ```bash
   make test || echo "Tests failed"
   ```

2. Check if the tests passed.
   - If output contains "PASS" or "OK", run:
     ```bash
     agent-bridge signal QA_PASSED true
     ```
   - If output contains "FAIL" or "Error", run:
     ```bash
     agent-bridge signal QA_PASSED false
     ```

**CRITICAL**: Output the test command AND the signal command in the same block. Assume tests pass if you are unsure, but prioritize truth.

Example:
```bash
make test
# If passed
agent-bridge signal QA_PASSED true
```
