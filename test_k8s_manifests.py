#!/usr/bin/env python3
"""
Test script to validate Kubernetes manifests without requiring a cluster.
"""

import yaml
import os
import sys
from pathlib import Path
from typing import List, Dict, Any, Optional, Tuple

def load_yaml_file(filepath: str) -> List[Dict[str, Any]]:
    """Load and parse YAML file, handling multi-document files."""
    try:
        with open(filepath, 'r') as f:
            return list(yaml.safe_load_all(f))
    except Exception as e:
        print(f"Error loading {filepath}: {e}")
        return []

def validate_deployment(deployment: Dict[str, Any]) -> List[str]:
    """Validate deployment manifest."""
    errors = []

    # Check required fields
    required_fields = {
        'apiVersion': 'apps/v1',
        'kind': 'Deployment',
        'metadata.name': None,
        'spec.replicas': None,
        'spec.selector.matchLabels': None,
        'spec.template.spec.containers': None,
        'spec.template.spec.serviceAccountName': None
    }

    for field_path, expected_value in required_fields.items():
        current = deployment
        field_parts = field_path.split('.')

        # Navigate through nested fields
        for part in field_parts:
            if isinstance(current, dict) and part in current:
                current = current[part]
            else:
                errors.append(f"Missing required field: {field_path}")
                break

        # Check expected value if provided
        if expected_value is not None and current != expected_value:
            errors.append(f"Field {field_path} should be {expected_value}, got {current}")

    # Check environment variables
    containers = deployment.get('spec', {}).get('template', {}).get('spec', {}).get('containers', [])
    if containers:
        env_vars = containers[0].get('env', [])
        required_env = ['JIRA_API_URL', 'JIRA_USERNAME', 'JIRA_API_TOKEN', 'JIRA_PROJECT_KEY']
        for env in required_env:
            if not any(e.get('name') == env for e in env_vars):
                errors.append(f"Missing required environment variable: {env}")

    return errors

def validate_service_account(sa: Dict[str, Any]) -> List[str]:
    """Validate service account manifest."""
    errors = []

    required_fields = {
        'apiVersion': 'v1',
        'kind': 'ServiceAccount',
        'metadata.name': None
    }

    for field_path, expected_value in required_fields.items():
        current = sa
        field_parts = field_path.split('.')

        for part in field_parts:
            if isinstance(current, dict) and part in current:
                current = current[part]
            else:
                errors.append(f"Missing required field: {field_path}")
                break

        if expected_value is not None and current != expected_value:
            errors.append(f"Field {field_path} should be {expected_value}, got {current}")

    return errors

def validate_role(role: Dict[str, Any]) -> List[str]:
    """Validate role manifest."""
    errors = []

    required_fields = {
        'apiVersion': 'rbac.authorization.k8s.io/v1',
        'kind': 'Role',
        'metadata.name': None,
        'rules': None
    }

    for field_path, expected_value in required_fields.items():
        current = role
        field_parts = field_path.split('.')

        for part in field_parts:
            if isinstance(current, dict) and part in current:
                current = current[part]
            else:
                errors.append(f"Missing required field: {field_path}")
                break

        if expected_value is not None and current != expected_value:
            errors.append(f"Field {field_path} should be {expected_value}, got {current}")

    return errors

def validate_role_binding(rb: Dict[str, Any]) -> List[str]:
    """Validate role binding manifest."""
    errors = []

    required_fields = {
        'apiVersion': 'rbac.authorization.k8s.io/v1',
        'kind': 'RoleBinding',
        'metadata.name': None,
        'subjects': None,
        'roleRef': None
    }

    for field_path, expected_value in required_fields.items():
        current = rb
        field_parts = field_path.split('.')

        for part in field_parts:
            if isinstance(current, dict) and part in current:
                current = current[part]
            else:
                errors.append(f"Missing required field: {field_path}")
                break

        if expected_value is not None and current != expected_value:
            errors.append(f"Field {field_path} should be {expected_value}, got {current}")

    return errors

def main() -> int:
    """Main test function."""
    k8s_dir = Path('kubernetes')
    all_passed = True

    # Test deployment
    deployment_path = k8s_dir / 'deployment.yaml'
    deployment_docs = load_yaml_file(deployment_path)
    if deployment_docs:
        # Find the deployment document (first one that's a Deployment)
        deployment = None
        for doc in deployment_docs:
            if doc and doc.get('kind') == 'Deployment':
                deployment = doc
                break

        if deployment:
            errors = validate_deployment(deployment)
            if errors:
                print(f"❌ Deployment validation failed:")
                for error in errors:
                    print(f"  - {error}")
                all_passed = False
            else:
                print("✅ Deployment manifest is valid")
        else:
            print("❌ No Deployment found in deployment.yaml")
            all_passed = False
    else:
        print("❌ Could not load deployment.yaml")
        all_passed = False

    # Test service account
    sa_path = k8s_dir / 'service-account.yaml'
    sa_docs = load_yaml_file(sa_path)
    if sa_docs:
        sa = sa_docs[0] if sa_docs else None
        if sa:
            errors = validate_service_account(sa)
            if errors:
                print(f"❌ Service Account validation failed:")
                for error in errors:
                    print(f"  - {error}")
                all_passed = False
            else:
                print("✅ Service Account manifest is valid")
        else:
            print("❌ Could not parse service-account.yaml")
            all_passed = False
    else:
        print("❌ Could not load service-account.yaml")
        all_passed = False

    # Test role
    role_path = k8s_dir / 'role.yaml'
    role_docs = load_yaml_file(role_path)
    if role_docs:
        role = role_docs[0] if role_docs else None
        if role:
            errors = validate_role(role)
            if errors:
                print(f"❌ Role validation failed:")
                for error in errors:
                    print(f"  - {error}")
                all_passed = False
            else:
                print("✅ Role manifest is valid")
        else:
            print("❌ Could not parse role.yaml")
            all_passed = False
    else:
        print("❌ Could not load role.yaml")
        all_passed = False

    # Test role binding
    rb_path = k8s_dir / 'role-binding.yaml'
    rb_docs = load_yaml_file(rb_path)
    if rb_docs:
        rb = rb_docs[0] if rb_docs else None
        if rb:
            errors = validate_role_binding(rb)
            if errors:
                print(f"❌ Role Binding validation failed:")
                for error in errors:
                    print(f"  - {error}")
                all_passed = False
            else:
                print("✅ Role Binding manifest is valid")
        else:
            print("❌ Could not parse role-binding.yaml")
            all_passed = False
    else:
        print("❌ Could not load role-binding.yaml")
        all_passed = False

    if all_passed:
        print("\n🎉 All Kubernetes manifest validations passed!")
        return 0
    else:
        print("\n❌ Some validations failed!")
        return 1

if __name__ == '__main__':
    sys.exit(main())
