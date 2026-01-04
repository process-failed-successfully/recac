/**
 * Dashboard JavaScript
 * Handles data fetching and UI updates
 */

document.addEventListener('DOMContentLoaded', function() {
    // Fetch and display dashboard data
    fetchDashboardData();

    // Set up periodic refresh
    setInterval(fetchDashboardData, 30000); // Refresh every 30 seconds
});

function fetchDashboardData() {
    // Fetch all data in parallel
    Promise.all([
        fetch('/api/status').then(res => res.json()),
        fetch('/api/tickets').then(res => res.json()),
        fetch('/api/metrics').then(res => res.json())
    ])
    .then(([status, tickets, metrics]) => {
        updateStatusOverview(status);
        updateMetrics(metrics);
        updateTicketsTable(tickets);
    })
    .catch(error => {
        console.error('Error fetching dashboard data:', error);
    });
}

function updateStatusOverview(status) {
    // Update system health
    const healthElement = document.getElementById('system-health');
    healthElement.textContent = status.system_health;
    healthElement.className = 'status-indicator ' + status.system_health;

    // Update tickets
    document.getElementById('total-tickets').textContent = status.total_tickets;
    document.getElementById('completed-tickets').textContent = status.completed_tickets;
    document.getElementById('pending-tickets').textContent = status.pending_tickets;

    // Update jobs
    document.getElementById('total-jobs').textContent = status.total_jobs;
    document.getElementById('completed-jobs').textContent = status.completed_jobs;
    document.getElementById('failed-jobs').textContent = status.failed_jobs;
    document.getElementById('pending-jobs').textContent = status.pending_jobs;

    // Update last updated time
    document.getElementById('last-updated-time').textContent =
        new Date(status.last_updated).toLocaleString();
}

function updateMetrics(metrics) {
    document.getElementById('cpu-usage').textContent = metrics.cpu_usage + '%';
    document.getElementById('memory-usage').textContent = metrics.memory_usage + '%';
    document.getElementById('disk-usage').textContent = metrics.disk_usage + '%';
    document.getElementById('active-workers').textContent = metrics.active_workers;
    document.getElementById('queue-size').textContent = metrics.queue_size;
    document.getElementById('uptime').textContent = metrics.uptime;
}

function updateTicketsTable(tickets) {
    const tableBody = document.getElementById('tickets-table-body');
    tableBody.innerHTML = '';

    if (tickets.length === 0) {
        const row = document.createElement('tr');
        row.innerHTML = '<td colspan="5" style="text-align: center;">No tickets found</td>';
        tableBody.appendChild(row);
        return;
    }

    tickets.forEach(ticket => {
        const row = document.createElement('tr');

        // ID
        const idCell = document.createElement('td');
        idCell.textContent = ticket.id;
        row.appendChild(idCell);

        // Title
        const titleCell = document.createElement('td');
        titleCell.textContent = ticket.title;
        row.appendChild(titleCell);

        // Status
        const statusCell = document.createElement('td');
        const statusBadge = document.createElement('span');
        statusBadge.textContent = ticket.status.replace('_', ' ');
        statusBadge.className = 'status-badge ' + ticket.status;
        statusCell.appendChild(statusBadge);
        row.appendChild(statusCell);

        // Created
        const createdCell = document.createElement('td');
        createdCell.textContent = new Date(ticket.created_at).toLocaleString();
        row.appendChild(createdCell);

        // Jobs
        const jobsCell = document.createElement('td');
        ticket.jobs.forEach(job => {
            const jobBadge = document.createElement('span');
            jobBadge.textContent = job.type + ':' + job.status;
            jobBadge.className = 'job-badge ' + job.status;
            jobsCell.appendChild(jobBadge);
        });
        row.appendChild(jobsCell);

        tableBody.appendChild(row);
    });
}
