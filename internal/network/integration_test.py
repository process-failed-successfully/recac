"""
Integration Test for Network Handler with Workflow

Demonstrates how the network handler integrates with the workflow system.
"""

import unittest
from unittest.mock import patch, MagicMock
from internal.network.network_handler import NetworkHandler, NetworkFailureHandler

class TestWorkflowIntegration(unittest.TestCase):
    """Test network handler integration with workflow."""

    def test_workflow_with_network_failure(self):
        """Test workflow behavior during network failures."""
        # Initialize network handlers
        network_handler = NetworkHandler(max_retries=2, base_delay=0.1)
        failure_handler = NetworkFailureHandler(network_handler)

        # Set fallback data for critical endpoints
        failure_handler.set_fallback_data(
            "http://api.example.com/tickets",
            {"tickets": [{"id": "fallback-123", "status": "pending"}]}
        )

        # Simulate workflow that needs to fetch tickets
        def workflow_fetch_tickets():
            """Simulate workflow fetching tickets from API."""
            # Try to fetch tickets with fallback
            tickets = failure_handler.get_with_cache("http://api.example.com/tickets")

            if tickets.get("status") == "error":
                # Handle error gracefully
                return {"status": "degraded", "message": "Using fallback data"}

            return {"status": "success", "tickets": tickets.get("tickets", [])}

        # Test 1: Successful network request
        with patch.object(network_handler, 'get') as mock_get:
            mock_get.return_value = {"tickets": [{"id": "123", "status": "active"}]}

            result = workflow_fetch_tickets()
            self.assertEqual(result["status"], "success")
            self.assertEqual(len(result["tickets"]), 1)

        # Test 2: Network failure with fallback
        with patch.object(network_handler, 'get') as mock_get:
            mock_get.return_value = None  # Simulate network failure

            result = workflow_fetch_tickets()
            self.assertEqual(result["status"], "degraded")

        # Test 3: Network failure with cached data
        with patch.object(network_handler, 'get') as mock_get:
            # First call succeeds, second fails
            mock_get.side_effect = [
                {"tickets": [{"id": "cached-123", "status": "active"}]},
                None
            ]

            # First fetch - successful
            result1 = workflow_fetch_tickets()
            self.assertEqual(result1["status"], "success")

            # Second fetch - uses cache
            result2 = workflow_fetch_tickets()
            self.assertEqual(result2["status"], "success")
            self.assertEqual(len(result2["tickets"]), 1)

if __name__ == "__main__":
    unittest.main()
