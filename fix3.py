with open("internal/orchestrator/api_generic_webhook_test.go", "r") as f:
    content = f.read()

content = content.replace('assert.Len(t, jobs, 1)\n\tassert.Equal(t, "generic-123", jobs[0].ID)', 'if assert.Len(t, jobs, 1) {\n\t\tassert.Equal(t, "generic-123", jobs[0].ID)\n\t}')
content = content.replace('assert.Len(t, jobs, 1)\n\tassert.Equal(t, "generic-secret-123", jobs[0].ID)', 'if assert.Len(t, jobs, 1) {\n\t\tassert.Equal(t, "generic-secret-123", jobs[0].ID)\n\t}')

with open("internal/orchestrator/api_generic_webhook_test.go", "w") as f:
    f.write(content)
