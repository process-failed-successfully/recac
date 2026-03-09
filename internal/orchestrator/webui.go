package orchestrator

const DashboardHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Orchestrator Dashboard</title>
    <style>
        body { font-family: sans-serif; margin: 0; padding: 0; background: #f4f4f4; color: #333; }
        .container { max-width: 1200px; margin: 0 auto; padding: 20px; }
        header { background: #333; color: #fff; padding: 10px 20px; display: flex; justify-content: space-between; align-items: center; }
        h1 { margin: 0; font-size: 1.5em; }
        .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; margin-top: 20px; }
        .card { background: #fff; padding: 20px; border-radius: 5px; box-shadow: 0 2px 5px rgba(0,0,0,0.1); }
        .card h2 { margin-top: 0; font-size: 1.2em; border-bottom: 1px solid #eee; padding-bottom: 10px; }
        .metric { display: flex; justify-content: space-between; margin-bottom: 10px; }
        .metric .label { font-weight: bold; color: #666; }
        .metric .value { font-family: monospace; }
        table { width: 100%; border-collapse: collapse; margin-top: 10px; }
        th, td { text-align: left; padding: 8px; border-bottom: 1px solid #ddd; }
        th { background-color: #f8f8f8; }
        .status-Completed { color: green; font-weight: bold; }
        .status-Failed { color: red; font-weight: bold; }
        .status-Running, .status-Active, .status-Spawning { color: blue; font-weight: bold; }
        .status-Pending { color: orange; font-weight: bold; }
        .actions { margin-top: 20px; display: flex; gap: 10px; }
        button { padding: 8px 16px; border: none; border-radius: 4px; background: #007bff; color: white; cursor: pointer; }
        button:hover { background: #0056b3; }
        button.danger { background: #dc3545; }
        button.danger:hover { background: #a82330; }
        #jobs-container { overflow-x: auto; }
        .controls { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px;}
        select, input { padding: 6px; }
    </style>
</head>
<body>
    <header>
        <h1>Orchestrator Dashboard</h1>
        <div id="connection-status">Connecting...</div>
    </header>
    <div class="container">
        <div class="actions" id="global-actions">
            <!-- Buttons will be injected here if supported -->
        </div>

        <div class="grid">
            <div class="card" id="status-card">
                <h2>Status</h2>
                <div id="status-content">Loading...</div>
            </div>
            <div class="card" id="analytics-card">
                <h2>Analytics</h2>
                <div id="analytics-content">Loading...</div>
            </div>
        </div>

        <div class="card" style="margin-top: 20px;">
            <div class="controls">
                <h2>Jobs</h2>
                <div>
                    <select id="job-state-filter">
                        <option value="">Active Jobs</option>
                        <option value="completed">Completed Jobs</option>
                        <option value="all">All Jobs</option>
                    </select>
                    <input type="text" id="job-search" placeholder="Search ID or Summary...">
                    <button id="refresh-jobs">Refresh</button>
                </div>
            </div>
            <div id="jobs-container">
                <table id="jobs-table">
                    <thead>
                        <tr>
                            <th>ID</th>
                            <th>Summary</th>
                            <th>Status</th>
                            <th>Start Time</th>
                            <th>Duration / Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr><td colspan="5">Loading jobs...</td></tr>
                    </tbody>
                </table>
            </div>
        </div>
    </div>
    <script>
        const formatDuration = (d) => {
            if (!d) return "0s";
            return d.replace(/h|m|s/g, match => match + " ").trim();
        };

        const formatDate = (ds) => {
            if (!ds || ds === "0001-01-01T00:00:00Z") return "N/A";
            return new Date(ds).toLocaleString();
        };

        const escapeHTML = (str) => {
            if (typeof str !== 'string') return str;
            return str.replace(/[&<>'"]/g,
                tag => ({
                    '&': '&amp;',
                    '<': '&lt;',
                    '>': '&gt;',
                    "'": '&#39;',
                    '"': '&quot;'
                }[tag])
            );
        };

        async function fetchStatus() {
            try {
                const res = await fetch('/status');
                const data = await res.json();
                document.getElementById('status-content').innerHTML = '<div class="metric"><span class="label">Uptime:</span> <span class="value">' + data.uptime + '</span></div>' +
                    '<div class="metric"><span class="label">Poll Interval:</span> <span class="value">' + data.poll_interval + '</span></div>' +
                    '<div class="metric"><span class="label">Last Poll:</span> <span class="value">' + formatDate(data.last_poll) + '</span></div>' +
                    '<div class="metric"><span class="label">Active Spawns:</span> <span class="value">' + data.active_spawns + '</span></div>' +
                    '<div class="metric"><span class="label">Pending Jobs:</span> <span class="value">' + data.pending_jobs + '</span></div>' +
                    '<div class="metric"><span class="label">Total Spawns:</span> <span class="value">' + data.total_spawns + '</span></div>' +
                    '<div class="metric"><span class="label">Max Concurrent:</span> <span class="value">' + (data.max_concurrent_jobs || 'Unlimited') + '</span></div>' +
                    '<div class="metric"><span class="label">State:</span> <span class="value" style="color: ' + (data.paused ? 'red' : 'green') + '">' + (data.paused ? 'PAUSED' : 'RUNNING') + ' ' + (data.draining ? '(DRAINING)' : '') + '</span></div>';

                let actionsHTML = '';
                if(data.paused) {
                    actionsHTML += '<button onclick="postAction(\'/resume\')">Resume</button>';
                } else {
                    actionsHTML += '<button onclick="postAction(\'/pause\')">Pause</button>';
                }

                if(data.draining) {
                    actionsHTML += '<button onclick="postAction(\'/undrain\')">Undrain</button>';
                } else {
                    actionsHTML += '<button class="danger" onclick="postAction(\'/drain\')">Drain</button>';
                }

                actionsHTML += '<button onclick="postAction(\'/poll\')">Force Poll</button>';
                actionsHTML += '<button class="danger" onclick="deleteAction(\'/pending\')">Clear Pending</button>';

                document.getElementById('global-actions').innerHTML = actionsHTML;
                document.getElementById('connection-status').innerText = 'Connected';
                document.getElementById('connection-status').style.color = 'lightgreen';
            } catch (err) {
                console.error('Error fetching status:', err);
                document.getElementById('connection-status').innerText = 'Disconnected';
                document.getElementById('connection-status').style.color = 'red';
            }
        }

        async function fetchAnalytics() {
            try {
                const res = await fetch('/analytics');
                const data = await res.json();
                document.getElementById('analytics-content').innerHTML = '<div class="metric"><span class="label">Total Jobs:</span> <span class="value">' + data.total_jobs + '</span></div>' +
                    '<div class="metric"><span class="label">Successful:</span> <span class="value" style="color: green">' + data.successful_jobs + '</span></div>' +
                    '<div class="metric"><span class="label">Failed:</span> <span class="value" style="color: red">' + data.failed_jobs + '</span></div>' +
                    '<div class="metric"><span class="label">Canceled:</span> <span class="value">' + data.canceled_jobs + '</span></div>' +
                    '<div class="metric"><span class="label">Success Rate:</span> <span class="value">' + data.success_rate.toFixed(2) + '%</span></div>' +
                    '<div class="metric"><span class="label">Avg Duration:</span> <span class="value">' + (data.average_duration/1e9).toFixed(2) + 's</span></div>';
            } catch (err) {
                console.error('Error fetching analytics:', err);
            }
        }

        async function fetchJobs() {
            try {
                const state = document.getElementById('job-state-filter').value;
                const search = document.getElementById('job-search').value.toLowerCase();

                let url = '/jobs';
                if (state) {
                    url += '?state=' + state;
                }

                const res = await fetch(url);
                let jobs = await res.json() || [];

                if (search) {
                    jobs = jobs.filter(j =>
                        j.id.toLowerCase().includes(search) ||
                        j.summary.toLowerCase().includes(search)
                    );
                }

                const tbody = document.querySelector('#jobs-table tbody');
                tbody.innerHTML = '';

                if (jobs.length === 0) {
                    tbody.innerHTML = '<tr><td colspan="5">No jobs found.</td></tr>';
                    return;
                }

                jobs.sort((a, b) => new Date(b.start_time) - new Date(a.start_time));

                jobs.forEach(j => {
                    const start = new Date(j.start_time);
                    let duration = "Running";
                    if (j.end_time && j.end_time !== "0001-01-01T00:00:00Z") {
                        const end = new Date(j.end_time);
                        duration = ((end - start) / 1000).toFixed(1) + "s";
                    } else if (start.getFullYear() > 2000) {
                        duration = ((new Date() - start) / 1000).toFixed(1) + "s (ongoing)";
                    } else {
                        duration = "Pending";
                    }

                    const safeId = escapeHTML(j.id);
                    const safeSummary = escapeHTML(j.summary.substring(0, 50) + (j.summary.length > 50 ? '...' : ''));
                    const safeStatus = escapeHTML(j.status);

                    let row = '<tr>' +
                        '<td><strong>' + safeId + '</strong></td>' +
                        '<td>' + safeSummary + '</td>' +
                        '<td class="status-' + safeStatus + '">' + safeStatus + '</td>' +
                        '<td>' + formatDate(j.start_time) + '</td>' +
                        '<td>' + duration + '</td>' +
                    '</tr>';
                    tbody.innerHTML += row;
                });
            } catch (err) {
                console.error('Error fetching jobs:', err);
            }
        }

        async function postAction(endpoint) {
            try {
                const res = await fetch(endpoint, { method: 'POST' });
                if(res.ok) {
                    fetchStatus();
                } else {
                    alert('Action failed: ' + await res.text());
                }
            } catch(e) {
                alert('Request failed: ' + e);
            }
        }

        async function deleteAction(endpoint) {
            if(!confirm('Are you sure you want to perform this delete action?')) return;
            try {
                const res = await fetch(endpoint, { method: 'DELETE' });
                if(res.ok) {
                    fetchStatus();
                    fetchJobs();
                } else {
                    alert('Action failed: ' + await res.text());
                }
            } catch(e) {
                alert('Request failed: ' + e);
            }
        }

        document.getElementById('refresh-jobs').addEventListener('click', fetchJobs);
        document.getElementById('job-state-filter').addEventListener('change', fetchJobs);
        document.getElementById('job-search').addEventListener('keyup', (e) => {
            if(e.key === 'Enter') fetchJobs();
        });

        // Init loops
        fetchStatus();
        fetchAnalytics();
        fetchJobs();

        setInterval(fetchStatus, 5000);
        setInterval(fetchAnalytics, 15000);
        setInterval(fetchJobs, 10000);
    </script>
</body>
</html>
`
