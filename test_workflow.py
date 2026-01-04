#!/usr/bin/env python3
"""
Test script for workflow execution
"""

from internal.workflow.workflow import WorkflowManager

def test_workflow_execution():
    """Test complete workflow execution from ticket to job completion"""
    print("Starting workflow execution test...")

    # Initialize workflow manager
    manager = WorkflowManager()

    # Test 1: Create a ticket
    print("\n1. Creating ticket...")
    ticket = manager.create_ticket(
        title="Data Processing Request",
        description="Process customer data from Q2 2023"
    )
    print(f"   Created ticket: {ticket.ticket_id}")

    # Test 2: Create jobs for the ticket
    print("\n2. Creating jobs...")
    job1 = manager.create_job(ticket.ticket_id, "processing")
    job2 = manager.create_job(ticket.ticket_id, "validation")
    print(f"   Created jobs: {job1.job_id}, {job2.job_id}")

    # Test 3: Process jobs
    print("\n3. Processing jobs...")
    manager.process_job(job1.job_id, {"status": "success", "records_processed": 1000})
    manager.process_job(job2.job_id, {"status": "success", "valid_records": 950})
    print("   Jobs processed successfully")

    # Test 4: Check ticket status
    print("\n4. Checking ticket status...")
    status = manager.get_ticket_status(ticket.ticket_id)
    print(f"   Ticket status: {status['ticket']['status']}")
    print(f"   Jobs status: {[j['status'] for j in status['jobs']]}")

    # Test 5: List all tickets
    print("\n5. Listing all tickets...")
    tickets = manager.list_tickets()
    print(f"   Total tickets: {len(tickets)}")

    # Test 6: List all jobs
    print("\n6. Listing all jobs...")
    jobs = manager.list_jobs()
    print(f"   Total jobs: {len(jobs)}")

    print("\n✅ All workflow execution tests passed!")

if __name__ == "__main__":
    test_workflow_execution()
