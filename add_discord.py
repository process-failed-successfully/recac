import json
import os

FEATURE_LIST_PATH = 'feature_list.json'

def load_features():
    if not os.path.exists(FEATURE_LIST_PATH):
        # Return empty structure if file doesn't exist
        return {"project_name": "Recac Project", "features": []}
    
    with open(FEATURE_LIST_PATH, 'r') as f:
        return json.load(f)

def save_features(data):
    with open(FEATURE_LIST_PATH, 'w') as f:
        json.dump(data, f, indent=2)

def add_discord_feature():
    data = load_features()
    
    new_feature = {
        "id": "discord-integration",
        "category": "backend",
        "description": "Discord notification integration",
        "status": "pending",
        "steps": ["Verify Discord webhook", "Check notification sent"],
        "dependencies": {
            "depends_on_ids": [],
            "exclusive_write_paths": ["internal/notify"],
            "read_only_paths": []
        }
    }

    features = data.get('features', [])
    
    exists = False
    for f in features:
        # Check by ID if available, or description
        if f.get('id') == 'discord-integration':
            exists = True
            break

    if not exists:
        # Insert after git-integration if possible, or append
        idx = len(features)
        for i, f in enumerate(features):
            if f.get('id') == 'git-integration':
                idx = i + 1 # Insert after
                break
        
        features.insert(idx, new_feature)
        data['features'] = features
        save_features(data)
        print("Added Discord integration feature.")
    else:
        print("Discord integration feature already exists.")

if __name__ == "__main__":
    add_discord_feature()
