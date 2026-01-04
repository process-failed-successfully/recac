"""
UI Dashboard for Workflow Status and Metrics

This module provides a web-based dashboard to visualize workflow execution status,
job metrics, and system health.
"""

import json
import os
from datetime import datetime
from flask import Flask, render_template, jsonify, request
from typing import Dict, List, Optional, Any

app = Flask(__name__,
            static_folder='static',
            template_folder='templates')

# Configuration
CONFIG_PATH = os.path.join(os.path.dirname(__file__), '../../config/workflow_config.json')

class DashboardData:
    """Handles data retrieval and processing for the dashboard"""

    def __init__(self):
        self.config = self._load_config()

    def _load_config(self) -> Dict[str, Any]:
        """Load workflow configuration"""
        try:
            with open(CONFIG_PATH, 'r') as f:
                return json.load(f)
        except Exception as e:
            print(f"Error loading config: {e}")
            return {}

    def get_workflow_status(self) -> Dict[str, Any]:
        """Get current workflow status and metrics"""
        # In a real implementation, this would connect to the workflow system
        # For now, we'll return mock data based on the config
        return {
            "total_tickets": 5,
            "completed_tickets": 3,
            "pending_tickets": 2,
            "total_jobs": 12,
            "completed_jobs": 8,
            "failed_jobs": 1,
            "pending_jobs": 3,
            "system_health": "healthy",
            "last_updated": datetime.now().isoformat()
        }

    def get_recent_tickets(self) -> List[Dict[str, Any]]:
        """Get recent tickets with their status"""
        return [
            {
                "id": "ticket-001",
                "title": "Data Processing Request",
                "status": "completed",
                "created_at": "2024-01-01T10:00:00",
                "completed_at": "2024-01-01T10:30:00",
                "jobs": [
                    {"id": "job-001", "type": "processing", "status": "completed"},
                    {"id": "job-002", "type": "validation", "status": "completed"}
                ]
            },
            {
                "id": "ticket-002",
                "title": "Data Validation Task",
                "status": "in_progress",
                "created_at": "2024-01-02T14:15:00",
                "jobs": [
                    {"id": "job-003", "type": "validation", "status": "running"},
                    {"id": "job-004", "type": "processing", "status": "pending"}
                ]
            }
        ]

    def get_system_metrics(self) -> Dict[str, Any]:
        """Get system performance metrics"""
        return {
            "cpu_usage": 45.2,
            "memory_usage": 68.7,
            "disk_usage": 32.1,
            "active_workers": 4,
            "queue_size": 2,
            "uptime": "2 days, 4 hours"
        }

# Initialize data handler
dashboard_data = DashboardData()

@app.route('/')
def index():
    """Main dashboard page"""
    return render_template('index.html')

@app.route('/api/status')
def api_status():
    """API endpoint for workflow status"""
    return jsonify(dashboard_data.get_workflow_status())

@app.route('/api/tickets')
def api_tickets():
    """API endpoint for recent tickets"""
    tickets = dashboard_data.get_recent_tickets()
    return jsonify(tickets)

@app.route('/api/metrics')
def api_metrics():
    """API endpoint for system metrics"""
    return jsonify(dashboard_data.get_system_metrics())

@app.route('/api/config')
def api_config():
    """API endpoint for workflow configuration"""
    return jsonify(dashboard_data.config)

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000, debug=True)
