"""
Network Handler Module

This module provides robust network operations with automatic retry logic,
timeout handling, and graceful error recovery for network failures.
"""

import requests
import time
import logging
from typing import Optional, Dict, Any, Callable
from requests.exceptions import RequestException, Timeout, ConnectionError

# Configure logging
logger = logging.getLogger(__name__)

class NetworkHandler:
    """
    A robust network handler that implements retry logic and error handling
    for HTTP requests.
    """

    def __init__(self, max_retries: int = 3, base_delay: float = 1.0,
                 timeout: float = 10.0):
        """
        Initialize the network handler.

        Args:
            max_retries: Maximum number of retry attempts
            base_delay: Base delay between retries in seconds
            timeout: Request timeout in seconds
        """
        self.max_retries = max_retries
        self.base_delay = base_delay
        self.timeout = timeout

    def make_request(self, method: str, url: str,
                    headers: Optional[Dict[str, str]] = None,
                    data: Optional[Dict[str, Any]] = None,
                    params: Optional[Dict[str, Any]] = None) -> Optional[Dict[str, Any]]:
        """
        Make an HTTP request with retry logic.

        Args:
            method: HTTP method (GET, POST, PUT, DELETE)
            url: Target URL
            headers: Request headers
            data: Request payload
            params: Query parameters

        Returns:
            Response data as dict if successful, None otherwise
        """
        if headers is None:
            headers = {}
        if data is None:
            data = {}
        if params is None:
            params = {}

        attempt = 0
        last_exception = None

        while attempt <= self.max_retries:
            attempt += 1
            try:
                logger.debug(f"Attempt {attempt} for {method} {url}")

                response = requests.request(
                    method=method.upper(),
                    url=url,
                    headers=headers,
                    json=data,
                    params=params,
                    timeout=self.timeout
                )

                # Check for successful response
                if response.status_code < 400:
                    try:
                        return response.json()
                    except ValueError:
                        return {"status": "success", "data": response.text}

                # Handle rate limiting
                if response.status_code == 429:
                    retry_after = int(response.headers.get('Retry-After', self.base_delay))
                    logger.warning(f"Rate limited. Retrying after {retry_after} seconds")
                    time.sleep(retry_after)
                    continue

                # Handle other client/server errors
                logger.error(f"Request failed with status {response.status_code}: {response.text}")
                return None

            except Timeout:
                last_exception = Timeout(f"Request timed out after {self.timeout} seconds")
                logger.warning(f"Timeout occurred: {last_exception}")
                self._handle_retry(attempt, last_exception)

            except ConnectionError as e:
                last_exception = e
                logger.error(f"Connection error: {str(e)}")
                self._handle_retry(attempt, last_exception)

            except RequestException as e:
                last_exception = e
                logger.error(f"Request exception: {str(e)}")
                self._handle_retry(attempt, last_exception)

            except Exception as e:
                last_exception = e
                logger.error(f"Unexpected error: {str(e)}")
                return None

        logger.error(f"All {self.max_retries} attempts failed. Last error: {str(last_exception)}")
        return None

    def _handle_retry(self, attempt: int, exception: Exception):
        """
        Handle retry logic with exponential backoff.

        Args:
            attempt: Current attempt number
            exception: The exception that occurred
        """
        if attempt <= self.max_retries:
            delay = self.base_delay * (2 ** (attempt - 1))
            logger.info(f"Retry {attempt}/{self.max_retries} in {delay:.2f} seconds...")
            time.sleep(delay)

    def get(self, url: str, headers: Optional[Dict[str, str]] = None,
            params: Optional[Dict[str, Any]] = None) -> Optional[Dict[str, Any]]:
        """
        Make a GET request.

        Args:
            url: Target URL
            headers: Request headers
            params: Query parameters

        Returns:
            Response data as dict if successful, None otherwise
        """
        return self.make_request("GET", url, headers, None, params)

    def post(self, url: str, headers: Optional[Dict[str, str]] = None,
             data: Optional[Dict[str, Any]] = None,
             params: Optional[Dict[str, Any]] = None) -> Optional[Dict[str, Any]]:
        """
        Make a POST request.

        Args:
            url: Target URL
            headers: Request headers
            data: Request payload
            params: Query parameters

        Returns:
            Response data as dict if successful, None otherwise
        """
        return self.make_request("POST", url, headers, data, params)

    def put(self, url: str, headers: Optional[Dict[str, str]] = None,
            data: Optional[Dict[str, Any]] = None,
            params: Optional[Dict[str, Any]] = None) -> Optional[Dict[str, Any]]:
        """
        Make a PUT request.

        Args:
            url: Target URL
            headers: Request headers
            data: Request payload
            params: Query parameters

        Returns:
            Response data as dict if successful, None otherwise
        """
        return self.make_request("PUT", url, headers, data, params)

    def delete(self, url: str, headers: Optional[Dict[str, str]] = None,
               params: Optional[Dict[str, Any]] = None) -> Optional[Dict[str, Any]]:
        """
        Make a DELETE request.

        Args:
            url: Target URL
            headers: Request headers
            params: Query parameters

        Returns:
            Response data as dict if successful, None otherwise
        """
        return self.make_request("DELETE", url, headers, None, params)

class NetworkFailureHandler:
    """
    Handles network failures and provides fallback mechanisms.
    """

    def __init__(self, network_handler: NetworkHandler):
        self.network_handler = network_handler
        self.fallback_data = {}

    def with_fallback(self, method: str, url: str,
                     fallback_data: Optional[Dict[str, Any]] = None,
                     headers: Optional[Dict[str, str]] = None,
                     data: Optional[Dict[str, Any]] = None,
                     params: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """
        Make a network request with fallback data if the request fails.

        Args:
            method: HTTP method
            url: Target URL
            fallback_data: Data to return if request fails
            headers: Request headers
            data: Request payload
            params: Query parameters

        Returns:
            Response data or fallback data
        """
        result = self.network_handler.make_request(method, url, headers, data, params)

        if result is None:
            logger.warning(f"Network request failed, using fallback data for {url}")
            if fallback_data:
                return fallback_data
            return {"status": "error", "message": "Network request failed", "fallback": True}

        return result

    def set_fallback_data(self, url: str, data: Dict[str, Any]):
        """
        Set fallback data for a specific URL.

        Args:
            url: URL to associate fallback data with
            data: Fallback data
        """
        self.fallback_data[url] = data

    def get_with_cache(self, url: str, headers: Optional[Dict[str, str]] = None,
                      params: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """
        Get data with caching mechanism.

        Args:
            url: Target URL
            headers: Request headers
            params: Query parameters

        Returns:
            Cached or fresh data
        """
        # Simple in-memory cache implementation
        cache_key = f"{url}_{str(params)}"
        if hasattr(self, '_cache') and cache_key in getattr(self, '_cache', {}):
            logger.debug(f"Returning cached data for {url}")
            return self._cache[cache_key]

        result = self.network_handler.get(url, headers, params)
        if result is None and url in self.fallback_data:
            result = self.fallback_data[url]

        if result is not None:
            if not hasattr(self, '_cache'):
                self._cache = {}
            self._cache[cache_key] = result

        return result if result is not None else {"status": "error", "message": "No data available"}

# Global network handler instance
network_handler = NetworkHandler()
network_failure_handler = NetworkFailureHandler(network_handler)
