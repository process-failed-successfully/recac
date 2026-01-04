"""
Network Handler Tests

Tests for network failure handling, retry logic, and fallback mechanisms.
"""

import unittest
import time
from unittest.mock import patch, MagicMock
from internal.network.network_handler import NetworkHandler, NetworkFailureHandler
import requests

class TestNetworkHandler(unittest.TestCase):
    """Test cases for NetworkHandler class."""

    def setUp(self):
        """Set up test fixtures."""
        self.handler = NetworkHandler(max_retries=2, base_delay=0.1, timeout=1.0)

    def test_successful_request(self):
        """Test successful HTTP request."""
        with patch('requests.request') as mock_request:
            mock_response = MagicMock()
            mock_response.status_code = 200
            mock_response.json.return_value = {"status": "success"}
            mock_request.return_value = mock_response

            result = self.handler.get("http://example.com")
            self.assertEqual(result, {"status": "success"})

    def test_retry_on_timeout(self):
        """Test retry logic on timeout."""
        with patch('requests.request') as mock_request:
            # First two attempts timeout, third succeeds
            mock_request.side_effect = [
                requests.exceptions.Timeout(),
                requests.exceptions.Timeout(),
                MagicMock(status_code=200, json=lambda: {"status": "success"})
            ]

            result = self.handler.get("http://example.com")
            self.assertEqual(result, {"status": "success"})
            self.assertEqual(mock_request.call_count, 3)

    def test_retry_on_connection_error(self):
        """Test retry logic on connection error."""
        with patch('requests.request') as mock_request:
            # First attempt fails, second succeeds
            mock_request.side_effect = [
                requests.exceptions.ConnectionError(),
                MagicMock(status_code=200, json=lambda: {"status": "success"})
            ]

            result = self.handler.get("http://example.com")
            self.assertEqual(result, {"status": "success"})
            self.assertEqual(mock_request.call_count, 2)

    def test_max_retries_exceeded(self):
        """Test behavior when max retries exceeded."""
        with patch('requests.request') as mock_request:
            mock_request.side_effect = requests.exceptions.Timeout()

            result = self.handler.get("http://example.com")
            self.assertIsNone(result)
            self.assertEqual(mock_request.call_count, 3)  # 1 initial + 2 retries

    def test_rate_limiting(self):
        """Test handling of rate limiting (429 status)."""
        with patch('requests.request') as mock_request:
            # First response is 429, second succeeds
            mock_response_429 = MagicMock()
            mock_response_429.status_code = 429
            mock_response_429.headers = {'Retry-After': '1'}

            mock_response_success = MagicMock()
            mock_response_success.status_code = 200
            mock_response_success.json.return_value = {"status": "success"}

            mock_request.side_effect = [mock_response_429, mock_response_success]

            result = self.handler.get("http://example.com")
            self.assertEqual(result, {"status": "success"})
            self.assertEqual(mock_request.call_count, 2)

    def test_http_error_handling(self):
        """Test handling of HTTP errors (4xx, 5xx)."""
        with patch('requests.request') as mock_request:
            mock_response = MagicMock()
            mock_response.status_code = 500
            mock_response.text = "Internal Server Error"
            mock_request.return_value = mock_response

            result = self.handler.get("http://example.com")
            self.assertIsNone(result)

    def test_post_request(self):
        """Test POST request."""
        with patch('requests.request') as mock_request:
            mock_response = MagicMock()
            mock_response.status_code = 201
            mock_response.json.return_value = {"id": 123}
            mock_request.return_value = mock_response

            result = self.handler.post("http://example.com", data={"key": "value"})
            self.assertEqual(result, {"id": 123})

class TestNetworkFailureHandler(unittest.TestCase):
    """Test cases for NetworkFailureHandler class."""

    def setUp(self):
        """Set up test fixtures."""
        self.network_handler = NetworkHandler(max_retries=1, base_delay=0.1)
        self.failure_handler = NetworkFailureHandler(self.network_handler)

    def test_with_fallback_success(self):
        """Test successful request with fallback available."""
        with patch.object(self.network_handler, 'make_request') as mock_request:
            mock_request.return_value = {"status": "success"}

            result = self.failure_handler.with_fallback(
                "GET", "http://example.com",
                fallback_data={"status": "fallback"}
            )
            self.assertEqual(result, {"status": "success"})

    def test_with_fallback_failure(self):
        """Test failed request using fallback data."""
        with patch.object(self.network_handler, 'make_request') as mock_request:
            mock_request.return_value = None

            result = self.failure_handler.with_fallback(
                "GET", "http://example.com",
                fallback_data={"status": "fallback", "cached": True}
            )
            self.assertEqual(result, {"status": "fallback", "cached": True})

    def test_with_fallback_no_fallback_data(self):
        """Test failed request with no fallback data."""
        with patch.object(self.network_handler, 'make_request') as mock_request:
            mock_request.return_value = None

            result = self.failure_handler.with_fallback(
                "GET", "http://example.com"
            )
            self.assertEqual(result["status"], "error")
            self.assertTrue(result["fallback"])

    def test_get_with_cache(self):
        """Test caching mechanism."""
        with patch.object(self.network_handler, 'get') as mock_get:
            mock_get.return_value = {"data": "fresh"}

            # First call - should fetch fresh data
            result1 = self.failure_handler.get_with_cache("http://example.com")
            self.assertEqual(result1, {"data": "fresh"})

            # Second call - should return cached data
            mock_get.return_value = None
            result2 = self.failure_handler.get_with_cache("http://example.com")
            self.assertEqual(result2, {"data": "fresh"})

    def test_get_with_cache_fallback(self):
        """Test cache with fallback data."""
        self.failure_handler.set_fallback_data("http://example.com", {"data": "fallback"})

        with patch.object(self.network_handler, 'get') as mock_get:
            mock_get.return_value = None

            result = self.failure_handler.get_with_cache("http://example.com")
            self.assertEqual(result, {"data": "fallback"})

if __name__ == "__main__":
    unittest.main()
