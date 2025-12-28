import json

with open('feature_list.json', 'r') as f:
    data = json.load(f)

new_feature = {
    "id": "discord-integration",
    "name": "Discord Integration",
    "description": "Discord notification integration",
    "status": "pending",
    "passes": False,
    "tests": ["internal/notify/discord_test.go"]
}

exists = False
for f in data['features']:
    if f['id'] == 'discord-integration':
        exists = True
        break

if not exists:
    idx = len(data['features'])
    for i, f in enumerate(data['features']):
        if f['id'] == 'git-integration':
            idx = i
            break
    data['features'].insert(idx, new_feature)

with open('feature_list.json', 'w') as f:
    json.dump(data, f, indent=2)
