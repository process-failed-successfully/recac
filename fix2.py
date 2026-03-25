with open("internal/orchestrator/api_generic_webhook_test.go", "r") as f:
    lines = f.readlines()

new_lines = []
for line in lines:
    if line.strip() == "assert.Len(t, jobs, 1)":
        new_lines.append("\tif assert.Len(t, jobs, 1) {\n")
    elif line.strip() == 'assert.Equal(t, "generic-123", jobs[0].ID)' or line.strip() == 'assert.Equal(t, "generic-secret-123", jobs[0].ID)':
        new_lines.append("\t\t" + line.strip() + "\n\t}\n")
    else:
        new_lines.append(line)

with open("internal/orchestrator/api_generic_webhook_test.go", "w") as f:
    for line in new_lines:
        f.write(line)
