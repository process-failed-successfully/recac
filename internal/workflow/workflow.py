"""
Workflow Execution Module

This module handles the complete workflow execution from ticket creation to job completion.
"""

import json
import uuid
from typing import Dict, List, Optional
from dataclasses import dataclass
from datetime import datetime
import logging

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

@dataclass
class Ticket:
    """Represents a workflow ticket"""
    ticket_id: str
    title: str
    description: str
    status: str = "created"
    created_at: str = datetime.now().isoformat()
    updated_at: str = datetime.now().isoformat()

@dataclass
class Job:
    """Represents a job in the workflow"""
    job_id: str
    ticket_id: str
    job_type: str
    status: str = "pending"
    result: Optional[Dict] = None
    created_at: str = datetime.now().isoformat()
    updated_at: str = datetime.now().isoformat()

class WorkflowManager:
    """Manages the complete workflow execution"""

    def __init__(self, config_path: str = "config/workflow_config.json"):
        self.config_path = config_path
        self.tickets: Dict[str, Ticket] = {}
        self.jobs: Dict[str, Job] = {}
        self._load_config()

    def _load_config(self):
        """Load workflow configuration"""
        try:
            with open(self.config_path, 'r') as f:
                self.config = json.load(f)
            logger.info("Workflow configuration loaded successfully")
        except FileNotFoundError:
            logger.warning(f"Config file not found at {self.config_path}, using defaults")
            self.config = {
                "max_tickets": 100,
                "job_types": ["processing", "validation", "notification"]
            }
        except json.JSONDecodeError:
            logger.error("Invalid JSON in config file")
            raise

    def create_ticket(self, title: str, description: str) -> Ticket:
        """Create a new workflow ticket"""
        if len(self.tickets) >= self.config.get("max_tickets", 100):
            raise ValueError("Maximum ticket limit reached")

        ticket_id = str(uuid.uuid4())
        ticket = Ticket(
            ticket_id=ticket_id,
            title=title,
            description=description
        )

        self.tickets[ticket_id] = ticket
        logger.info(f"Created ticket {ticket_id}: {title}")
        return ticket

    def create_job(self, ticket_id: str, job_type: str) -> Job:
        """Create a job for a ticket"""
        if ticket_id not in self.tickets:
            raise ValueError(f"Ticket {ticket_id} not found")

        if job_type not in self.config.get("job_types", []):
            raise ValueError(f"Invalid job type: {job_type}")

        job_id = str(uuid.uuid4())
        job = Job(
            job_id=job_id,
            ticket_id=ticket_id,
            job_type=job_type
        )

        self.jobs[job_id] = job
        logger.info(f"Created job {job_id} for ticket {ticket_id} (type: {job_type})")
        return job

    def process_job(self, job_id: str, result: Dict) -> Job:
        """Process a job and mark it as completed"""
        if job_id not in self.jobs:
            raise ValueError(f"Job {job_id} not found")

        job = self.jobs[job_id]
        job.status = "completed"
        job.result = result
        job.updated_at = datetime.now().isoformat()

        # Update ticket status if all jobs are completed
        ticket_id = job.ticket_id
        ticket_jobs = [j for j in self.jobs.values() if j.ticket_id == ticket_id]
        if all(j.status == "completed" for j in ticket_jobs):
            self.tickets[ticket_id].status = "completed"
            self.tickets[ticket_id].updated_at = datetime.now().isoformat()
            logger.info(f"Ticket {ticket_id} completed as all jobs finished")

        logger.info(f"Job {job_id} processed successfully")
        return job

    def get_ticket_status(self, ticket_id: str) -> Dict:
        """Get the status of a ticket and its jobs"""
        if ticket_id not in self.tickets:
            raise ValueError(f"Ticket {ticket_id} not found")

        ticket = self.tickets[ticket_id]
        jobs = [j for j in self.jobs.values() if j.ticket_id == ticket_id]

        return {
            "ticket": {
                "id": ticket.ticket_id,
                "title": ticket.title,
                "status": ticket.status,
                "created_at": ticket.created_at,
                "updated_at": ticket.updated_at
            },
            "jobs": [{
                "id": j.job_id,
                "type": j.job_type,
                "status": j.status,
                "created_at": j.created_at,
                "updated_at": j.updated_at
            } for j in jobs]
        }

    def list_tickets(self) -> List[Dict]:
        """List all tickets with their status"""
        return [{
            "id": t.ticket_id,
            "title": t.title,
            "status": t.status,
            "created_at": t.created_at
        } for t in self.tickets.values()]

    def list_jobs(self, ticket_id: Optional[str] = None) -> List[Dict]:
        """List all jobs, optionally filtered by ticket_id"""
        jobs = self.jobs.values()
        if ticket_id:
            jobs = [j for j in jobs if j.ticket_id == ticket_id]

        return [{
            "id": j.job_id,
            "ticket_id": j.ticket_id,
            "type": j.job_type,
            "status": j.status,
            "created_at": j.created_at
        } for j in jobs]
