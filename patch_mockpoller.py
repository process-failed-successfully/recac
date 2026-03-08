import sys

def main():
    with open("internal/orchestrator/orchestrator_test.go", "r") as f:
        content = f.read()

    old = """func (m *mockPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	m.updateStatusMu.Lock()
	defer m.updateStatusMu.Unlock()
	m.updateStatus[item.ID] = status
	return nil
}"""

    new = """func (m *mockPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	m.updateStatusMu.Lock()
	defer m.updateStatusMu.Unlock()
	if m.updateStatus == nil {
		m.updateStatus = make(map[string]string)
	}
	m.updateStatus[item.ID] = status
	return nil
}"""

    content = content.replace(old, new)

    with open("internal/orchestrator/orchestrator_test.go", "w") as f:
        f.write(content)

if __name__ == "__main__":
    main()
