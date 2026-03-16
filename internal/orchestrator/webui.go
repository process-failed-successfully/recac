package orchestrator

const DashboardHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Orchestrator Dashboard</title>
    <script src="https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.min.js"></script>
    <script>
        mermaid.initialize({ startOnLoad: false });
    </script>
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
        button { padding: 8px 16px; border: none; border-radius: 4px; background: #007bff; color: white; cursor: pointer; transition: all 0.2s ease; }
        button:hover { background: #0056b3; }
        button:disabled { opacity: 0.65; cursor: not-allowed; }
        button:focus-visible { outline: 2px solid #007bff; outline-offset: 2px; }
        .modal { display: none; position: fixed; z-index: 1000; left: 0; top: 0; width: 100%; height: 100%; overflow: auto; background-color: rgba(0,0,0,0.4); }
        .modal-content { background-color: #fefefe; margin: 10% auto; padding: 20px; border: 1px solid #888; width: 80%; max-width: 600px; border-radius: 5px; }
        .close { background: none; border: none; padding: 0; color: #aaa; float: right; font-size: 28px; font-weight: bold; cursor: pointer; }
        .close:hover, .close:focus { color: black; text-decoration: none; cursor: pointer; outline: none; }
        .close:focus-visible { outline: 2px solid #007bff; outline-offset: 2px; border-radius: 2px; }
        .form-group { margin-bottom: 15px; }
        .form-group label { display: block; margin-bottom: 5px; font-weight: bold; }
        .form-group input[type="text"], .form-group textarea { width: 100%; padding: 8px; border: 1px solid #ccc; border-radius: 4px; box-sizing: border-box; }
        .form-group textarea { resize: vertical; height: 100px; }
        button.danger { background: #dc3545; }
        button.danger:hover { background: #a82330; }
        #jobs-container { overflow-x: auto; }
        .controls { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px;}
        select, input { padding: 6px; }
        #logs-output { background: #222; color: #ddd; padding: 15px; border-radius: 4px; font-family: monospace; white-space: pre-wrap; overflow-y: auto; height: 400px; margin: 0; }
        .modal-large { width: 90%; max-width: 1000px; }
    </style>
</head>
<body>
    <header>
        <h1>Orchestrator Dashboard</h1>
        <div id="connection-status">Connecting...</div>
    </header>
    <div class="container">
        <div class="controls" style="margin-top: 20px;">
            <div class="actions" id="global-actions" style="margin-top: 0;">
                <!-- Buttons will be injected here if supported -->
            </div>
            <div>
                <button type="button" onclick="openSearchLogsModal()" aria-label="Search Logs" style="background-color: #6c757d; margin-right: 10px;">Search Logs</button>
                <button type="button" onclick="viewGraph()" style="background-color: #6f42c1; margin-right: 10px;">View Graph</button>
                <button type="button" onclick="document.getElementById('submitPipelineModal').style.display='block'" style="background-color: #17a2b8; margin-right: 10px;">+ Submit Pipeline</button>
                <button type="button" onclick="document.getElementById('submitModal').style.display='block'" style="background-color: #28a745;">+ Submit Job</button>
            </div>
        </div>

        <div id="submitPipelineModal" class="modal">
            <div class="modal-content">
                <button type="button" class="close" aria-label="Close modal" onclick="document.getElementById('submitPipelineModal').style.display='none'">&times;</button>
                <h2>Submit Pipeline (YAML)</h2>
                <div class="form-group">
                    <label for="pipeline-yaml">Pipeline Definition</label>
                    <textarea id="pipeline-yaml" placeholder="name: my-pipeline&#10;jobs:&#10;  ..." style="height: 300px; font-family: monospace;"></textarea>
                </div>
                <div style="display: flex; gap: 10px;">
                    <button type="button" id="btn-dry-run" onclick="dryRunPipeline()" style="background-color: #6c757d; flex: 1;">Dry Run</button>
                    <button type="button" id="btn-submit-pipeline" onclick="submitPipeline()" style="background-color: #17a2b8; flex: 1;">Submit Pipeline</button>
                </div>
                <div id="dry-run-results" style="display: none; margin-top: 15px; padding: 10px; background: #f8f9fa; border: 1px solid #dee2e6; border-radius: 4px; max-height: 200px; overflow-y: auto;">
                    <h3 style="margin-top: 0; font-size: 1.1em;">Dry Run Results</h3>
                    <pre id="dry-run-output" style="margin: 0; font-size: 0.9em; white-space: pre-wrap;"></pre>
                </div>
            </div>
        </div>

        <div id="submitModal" class="modal">
            <div class="modal-content">
                <button type="button" class="close" aria-label="Close modal" onclick="document.getElementById('submitModal').style.display='none'">&times;</button>
                <h2>Submit Ad-hoc Job</h2>
                <div class="form-group">
                    <label for="job-id">Job ID (Optional, auto-generated if empty)</label>
                    <input type="text" id="job-id" placeholder="e.g., MY-JOB-123">
                </div>
                <div class="form-group">
                    <label for="job-summary">Summary *</label>
                    <input type="text" id="job-summary" placeholder="e.g., Fix login bug">
                </div>
                <div class="form-group">
                    <label for="job-repo">Repository URL *</label>
                    <input type="text" id="job-repo" placeholder="e.g., https://github.com/org/repo">
                </div>
                <div class="form-group">
                    <label for="job-deps">Depends On (Optional, comma-separated IDs)</label>
                    <input type="text" id="job-deps" placeholder="e.g., JOB-1, JOB-2">
                </div>
                <div class="form-group">
                    <label for="job-tags">Tags (Optional, comma-separated tags)</label>
                    <input type="text" id="job-tags" placeholder="e.g., bug, frontend">
                </div>
                <div class="form-group" style="display: flex; gap: 10px; align-items: center;">
                    <div style="flex: 1;">
                        <label for="job-concurrency-group">Concurrency Group (Optional)</label>
                        <input type="text" id="job-concurrency-group" placeholder="e.g., deploy-prod">
                    </div>
                    <div style="display: flex; align-items: center; gap: 5px; margin-top: 15px;">
                        <input type="checkbox" id="job-cancel-in-progress" style="width: auto;">
                        <label for="job-cancel-in-progress" style="margin-bottom: 0;">Cancel In Progress</label>
                    </div>
                </div>
                <div class="form-group">
                    <label for="job-agent-provider">Agent Provider (Optional)</label>
                    <input type="text" id="job-agent-provider" placeholder="e.g., openrouter">
                </div>
                <div class="form-group">
                    <label for="job-agent-model">Agent Model (Optional)</label>
                    <input type="text" id="job-agent-model" placeholder="e.g., openai/gpt-4o-mini">
                </div>
                <div class="form-group">
                    <label for="job-env">Environment Variables (Optional, KEY=VALUE, one per line)</label>
                    <textarea id="job-env" placeholder="DEBUG=true&#10;PORT=8080"></textarea>
                </div>
                <div class="form-group">
                    <label for="job-desc">Description (Optional)</label>
                    <textarea id="job-desc" placeholder="Detailed description of the task..."></textarea>
                </div>
                <button type="button" id="btn-submit-adhoc" onclick="submitAdHocJob()" style="background-color: #28a745; width: 100%;">Submit Job</button>
            </div>
        </div>

        <div id="editDepsModal" class="modal">
            <div class="modal-content">
                <button type="button" class="close" aria-label="Close modal" onclick="document.getElementById('editDepsModal').style.display='none'">&times;</button>
                <h2>Edit Dependencies for <span id="edit-deps-job-id-display"></span></h2>
                <input type="hidden" id="edit-deps-job-id">
                <textarea id="edit-deps-input" placeholder="JOB-1, JOB-2" style="width: 100%; height: 60px; margin-bottom: 10px;"></textarea>
                <button type="button" id="btn-submit-deps" onclick="submitEditDeps()" style="background-color: #007bff; width: 100%;">Save Dependencies</button>
            </div>
        </div>

        <div id="logsModal" class="modal">
            <div class="modal-content modal-large">
                <button type="button" class="close" aria-label="Close modal" onclick="closeLogs()">&times;</button>
                <h2 id="logs-title">Job Logs</h2>
                <pre id="logs-output"></pre>
            </div>
        </div>

        <div id="searchLogsModal" class="modal">
            <div class="modal-content modal-large">
                <button type="button" class="close" aria-label="Close modal" onclick="closeSearchLogsModal()">&times;</button>
                <h2>Search Logs</h2>
                <div style="display: flex; gap: 10px; margin-bottom: 15px;">
                    <div class="form-group" style="flex: 2; margin-bottom: 0;">
                        <input type="text" id="search-logs-query" placeholder="Regex query (e.g., panic, error)...">
                    </div>
                    <div class="form-group" style="flex: 1; margin-bottom: 0;">
                        <input type="text" id="search-logs-tag" placeholder="Filter by tag (optional)">
                    </div>
                    <div class="form-group" style="flex: 1; margin-bottom: 0;">
                        <select id="search-logs-status" style="width: 100%; border: 1px solid #ccc; border-radius: 4px; padding: 8px;">
                            <option value="">Any Status</option>
                            <option value="Completed">Completed</option>
                            <option value="Failed">Failed</option>
                            <option value="Running">Running</option>
                            <option value="Canceled">Canceled</option>
                        </select>
                    </div>
                    <button type="button" onclick="performSearchLogs()" style="background-color: #007bff; min-width: 100px;">Search</button>
                </div>
                <div id="search-logs-results" style="max-height: 500px; overflow-y: auto; background: #222; color: #ddd; padding: 15px; border-radius: 4px; font-family: monospace; display: none;">
                    <!-- Results will be injected here -->
                </div>
            </div>
        </div>

        <div id="graphModal" class="modal">
            <div class="modal-content modal-large" style="width: 95%; max-width: 1400px; height: 90vh; display: flex; flex-direction: column;">
                <button type="button" class="close" aria-label="Close modal" onclick="closeGraph()">&times;</button>
                <h2 style="margin-bottom: 0;">Dependency Graph</h2>
                <div id="graphDiv" style="flex: 1; overflow: auto; display: flex; justify-content: center; align-items: center; background: #fff; border: 1px solid #ccc; border-radius: 4px; margin-top: 15px;">
                    Loading graph...
                </div>
            </div>
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
                    <select id="job-state-filter" aria-label="Filter jobs by state">
                        <option value="">Active Jobs</option>
                        <option value="completed">Completed Jobs</option>
                        <option value="all">All Jobs</option>
                    </select>
                    <input type="text" id="job-search" placeholder="Search ID or Summary..." aria-label="Search jobs">
                    <button type="button" id="refresh-jobs">Refresh</button>
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
                    actionsHTML += '<button type="button" onclick="postAction(\'/resume\')">Resume</button>';
                } else {
                    actionsHTML += '<button type="button" onclick="postAction(\'/pause\')">Pause</button>';
                }

                if(data.draining) {
                    actionsHTML += '<button type="button" onclick="postAction(\'/undrain\')">Undrain</button>';
                } else {
                    actionsHTML += '<button type="button" class="danger" onclick="postAction(\'/drain\')">Drain</button>';
                }

                actionsHTML += '<button type="button" onclick="postAction(\'/poll\')">Force Poll</button>';
                actionsHTML += '<button type="button" class="danger" onclick="deleteAction(\'/pending\')">Clear Pending</button>';

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
                let html = '<div class="metric"><span class="label">Total Jobs:</span> <span class="value">' + data.total_jobs + '</span></div>' +
                    '<div class="metric"><span class="label">Successful:</span> <span class="value" style="color: green">' + data.successful_jobs + '</span></div>' +
                    '<div class="metric"><span class="label">Failed:</span> <span class="value" style="color: red">' + data.failed_jobs + '</span></div>' +
                    '<div class="metric"><span class="label">Canceled:</span> <span class="value">' + data.canceled_jobs + '</span></div>' +
                    '<div class="metric"><span class="label">Success Rate:</span> <span class="value">' + data.success_rate.toFixed(2) + '%</span></div>' +
                    '<div class="metric"><span class="label">Avg Duration:</span> <span class="value">' + (data.average_duration/1e9).toFixed(2) + 's</span></div>';

                if (data.total_metrics && Object.keys(data.total_metrics).length > 0) {
                    html += '<div style="margin-top: 10px; border-top: 1px solid #eee; padding-top: 10px;"><strong>Total Metrics</strong></div>';
                    for (const [key, value] of Object.entries(data.total_metrics)) {
                        html += '<div class="metric"><span class="label">' + escapeHTML(key) + ':</span> <span class="value">' + value.toFixed(2) + '</span></div>';
                    }
                }

                document.getElementById('analytics-content').innerHTML = html;
            } catch (err) {
                console.error('Error fetching analytics:', err);
            }
        }

        async function doJobAction(action, id) {
            let method = 'POST';
            let url = '';

            if (action === 'approve') {
                url = '/jobs/' + encodeURIComponent(id) + '/approve';
            } else if (action === 'retry') {
                url = '/jobs/' + encodeURIComponent(id) + '/retry';
            } else if (action === 'cancel') {
                if(!confirm('Are you sure you want to cancel job ' + id + '?')) return;
                url = '/jobs/' + encodeURIComponent(id);
                method = 'DELETE';
            } else if (action === 'purge') {
                if(!confirm('Are you sure you want to purge job ' + id + '?')) return;
                url = '/history/' + encodeURIComponent(id);
                method = 'DELETE';
            }

            try {
                const res = await fetch(url, { method: method });
                if(res.ok) {
                    fetchStatus();
                    fetchJobs();
                } else {
                    alert('Action ' + action + ' failed: ' + await res.text());
                }
            } catch(e) {
                alert('Request failed: ' + e);
            }
        }

        function cloneJob(encodedJobJson) {
            try {
                const j = JSON.parse(decodeURIComponent(encodedJobJson));
                document.getElementById('job-id').value = '';
                document.getElementById('job-summary').value = j.summary || '';
                document.getElementById('job-repo').value = j.work_item.repo_url || '';
                document.getElementById('job-deps').value = (j.work_item.depends_on || []).join(', ');
                document.getElementById('job-tags').value = (j.work_item.tags || []).join(', ');
                document.getElementById('job-concurrency-group').value = j.work_item.concurrency_group || '';
                document.getElementById('job-cancel-in-progress').checked = j.work_item.cancel_in_progress || false;
                document.getElementById('job-agent-provider').value = j.work_item.agent_provider || '';
                document.getElementById('job-agent-model').value = j.work_item.agent_model || '';

                let envStr = '';
                if (j.work_item.env_vars) {
                    for (const [key, val] of Object.entries(j.work_item.env_vars)) {
                        envStr += key + '=' + val + '\n';
                    }
                }
                document.getElementById('job-env').value = envStr;
                document.getElementById('job-desc').value = j.work_item.description || '';

                document.getElementById('submitModal').style.display = 'block';
            } catch (e) {
                console.error("Error cloning job:", e);
                alert("Failed to clone job details.");
            }
        }

        function editDependencies(encodedJobJson) {
            try {
                const j = JSON.parse(decodeURIComponent(encodedJobJson));
                document.getElementById('edit-deps-job-id').value = j.id;
                document.getElementById('edit-deps-job-id-display').innerText = j.id;
                document.getElementById('edit-deps-input').value = (j.work_item.depends_on || []).join(', ');
                document.getElementById('editDepsModal').style.display = 'block';
            } catch (e) {
                console.error("Error editing dependencies:", e);
                alert("Failed to load job details.");
            }
        }

        async function submitEditDeps() {
            const btn = document.getElementById('btn-submit-deps');
            btn.disabled = true;
            btn.innerText = 'Saving...';

            const id = document.getElementById('edit-deps-job-id').value;
            const depsStr = document.getElementById('edit-deps-input').value.trim();
            const deps = depsStr ? depsStr.split(',').map(s => s.trim()).filter(s => s) : [];

            try {
                const res = await fetch('/jobs/' + encodeURIComponent(id) + '/dependencies', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ depends_on: deps })
                });
                if (res.ok) {
                    document.getElementById('editDepsModal').style.display = 'none';
                    fetchStatus();
                    fetchJobs();
                } else {
                    alert('Failed to update dependencies: ' + await res.text());
                }
            } catch(e) {
                alert('Request failed: ' + e);
            } finally {
                btn.disabled = false;
                btn.innerText = 'Save Dependencies';
            }
        }

        async function dryRunPipeline() {
            const yaml = document.getElementById('pipeline-yaml').value.trim();
            if (!yaml) {
                alert('Pipeline YAML is required.');
                return;
            }

            const btn = document.getElementById('btn-dry-run');
            btn.disabled = true;
            btn.innerText = 'Running...';

            try {
                const res = await fetch('/jobs/pipeline/dry-run', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/x-yaml' },
                    body: yaml
                });

                const resultsDiv = document.getElementById('dry-run-results');
                const outputPre = document.getElementById('dry-run-output');

                if (res.ok) {
                    const items = await res.json();
                    let outputText = "Generated " + (items ? items.length : 0) + " jobs:\n\n";
                    if (items) {
                        items.forEach((item, index) => {
                            outputText += (index + 1) + ". " + item.id + "\n";
                            outputText += "   Summary: " + item.summary + "\n";
                            if (item.depends_on && item.depends_on.length > 0) {
                                outputText += "   Depends On: " + item.depends_on.join(", ") + "\n";
                            }
                        });
                    }
                    outputPre.innerText = outputText;
                    resultsDiv.style.display = 'block';
                } else {
                    const text = await res.text();
                    outputPre.innerText = "Error: " + text;
                    resultsDiv.style.display = 'block';
                }
            } catch(e) {
                alert('Request failed: ' + e);
            } finally {
                btn.disabled = false;
                btn.innerText = 'Dry Run';
            }
        }

        async function submitPipeline() {
            const yaml = document.getElementById('pipeline-yaml').value.trim();
            if (!yaml) {
                alert('Pipeline YAML is required.');
                return;
            }

            const btn = document.getElementById('btn-submit-pipeline');
            btn.disabled = true;
            btn.innerText = 'Submitting...';

            try {
                const res = await fetch('/jobs/pipeline', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/x-yaml' },
                    body: yaml
                });

                if (res.ok) {
                    document.getElementById('submitPipelineModal').style.display = 'none';
                    document.getElementById('pipeline-yaml').value = '';
                    fetchStatus();
                    fetchJobs();
                } else {
                    alert('Failed to submit pipeline: ' + await res.text());
                }
            } catch(e) {
                alert('Request failed: ' + e);
            } finally {
                btn.disabled = false;
                btn.innerText = 'Submit Pipeline';
            }
        }

        async function submitAdHocJob() {
            const summary = document.getElementById('job-summary').value.trim();
            const repo = document.getElementById('job-repo').value.trim();

            if (!summary || !repo) {
                alert('Summary and Repository URL are required.');
                return;
            }

            const btn = document.getElementById('btn-submit-adhoc');
            btn.disabled = true;
            btn.innerText = 'Submitting...';

            const id = document.getElementById('job-id').value.trim();
            const depsStr = document.getElementById('job-deps').value.trim();
            const tagsStr = document.getElementById('job-tags').value.trim();
            const concurrencyGroup = document.getElementById('job-concurrency-group').value.trim();
            const cancelInProgress = document.getElementById('job-cancel-in-progress').checked;
            const agentProvider = document.getElementById('job-agent-provider').value.trim();
            const agentModel = document.getElementById('job-agent-model').value.trim();
            const envStr = document.getElementById('job-env').value.trim();
            const desc = document.getElementById('job-desc').value.trim();

            const deps = depsStr ? depsStr.split(',').map(s => s.trim()).filter(s => s) : [];
            const tags = tagsStr ? tagsStr.split(',').map(s => s.trim()).filter(s => s) : [];
            const env = {};
            if (envStr) {
                const lines = envStr.split('\n');
                for (const line of lines) {
                    const parts = line.split('=');
                    if (parts.length >= 2) {
                        const key = parts[0].trim();
                        const val = parts.slice(1).join('=').trim();
                        if (key) env[key] = val;
                    }
                }
            }

            const payload = {
                id: id || ('adhoc-' + Math.random().toString(36).substr(2, 9)),
                summary: summary,
                repo_url: repo,
                description: desc,
                depends_on: deps,
                tags: tags,
                concurrency_group: concurrencyGroup,
                cancel_in_progress: cancelInProgress,
                agent_provider: agentProvider,
                agent_model: agentModel,
                env_vars: env
            };

            try {
                const res = await fetch('/jobs', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload)
                });

                if (res.ok) {
                    document.getElementById('submitModal').style.display = 'none';
                    // Reset form
                    document.getElementById('job-id').value = '';
                    document.getElementById('job-summary').value = '';
                    document.getElementById('job-repo').value = '';
                    document.getElementById('job-deps').value = '';
                    document.getElementById('job-tags').value = '';
                    document.getElementById('job-concurrency-group').value = '';
                    document.getElementById('job-cancel-in-progress').checked = false;
                    document.getElementById('job-agent-provider').value = '';
                    document.getElementById('job-agent-model').value = '';
                    document.getElementById('job-env').value = '';
                    document.getElementById('job-desc').value = '';
                    fetchStatus();
                    fetchJobs();
                } else {
                    alert('Failed to submit job: ' + await res.text());
                }
            } catch(e) {
                alert('Request failed: ' + e);
            } finally {
                btn.disabled = false;
                btn.innerText = 'Submit Job';
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
                    tbody.innerHTML = '<tr><td colspan="5" style="text-align: center; padding: 2em; color: #666;">No jobs found.<br><br><button type="button" onclick="document.getElementById(\'submitModal\').style.display=\'block\'" style="background-color: #28a745;">+ Submit Job</button></td></tr>';
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

                    let actionButtons = '';
                    const lowerStatus = (j.status || '').toLowerCase();

                    if (lowerStatus === 'pending approval') {
                        actionButtons += '<button type="button" style="margin-left:10px; padding:4px 8px; font-size:12px;" onclick="doJobAction(\'approve\', \'' + escapeHTML(j.id) + '\')">Approve</button>';
                    } else if (lowerStatus === 'failed') {
                        actionButtons += '<button type="button" style="margin-left:10px; padding:4px 8px; font-size:12px;" onclick="doJobAction(\'retry\', \'' + escapeHTML(j.id) + '\')">Retry</button>';
                    }

                    if (lowerStatus === 'running' || lowerStatus === 'spawning' || lowerStatus === 'active' || lowerStatus === 'pending') {
                        actionButtons += '<button type="button" class="danger" style="margin-left:10px; padding:4px 8px; font-size:12px;" onclick="doJobAction(\'cancel\', \'' + escapeHTML(j.id) + '\')">Cancel</button>';
                    }

                    if (lowerStatus === 'completed' || lowerStatus === 'failed' || lowerStatus === 'canceled' || lowerStatus === 'error') {
                        actionButtons += '<button type="button" class="danger" style="margin-left:10px; padding:4px 8px; font-size:12px;" onclick="doJobAction(\'purge\', \'' + escapeHTML(j.id) + '\')">Purge</button>';
                    }

                    actionButtons += '<button type="button" style="margin-left:10px; padding:4px 8px; font-size:12px; background-color: #6c757d;" onclick="viewLogs(\'' + escapeHTML(j.id) + '\')">Logs</button>';
                    const safeJobJson = encodeURIComponent(JSON.stringify(j)).replace(/'/g, "%27");
                    if (lowerStatus === 'pending') {
                        actionButtons += '<button type="button" style="margin-left:10px; padding:4px 8px; font-size:12px; background-color: #ffc107; color: #212529;" onclick="editDependencies(\'' + safeJobJson + '\')">Set Deps</button>';
                    }
                    actionButtons += '<button type="button" style="margin-left:10px; padding:4px 8px; font-size:12px; background-color: #17a2b8;" onclick="cloneJob(\'' + safeJobJson + '\')">Clone</button>';

                    let row = '<tr>' +
                        '<td><strong>' + safeId + '</strong></td>' +
                        '<td>' + safeSummary + '</td>' +
                        '<td class="status-' + safeStatus + '">' + safeStatus + '</td>' +
                        '<td>' + formatDate(j.start_time) + '</td>' +
                        '<td>' + duration + actionButtons + '</td>' +
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

        let currentLogController = null;

        async function viewLogs(id) {
            document.getElementById('logsModal').style.display = 'block';
            document.getElementById('logs-title').innerText = 'Logs for ' + id;
            const output = document.getElementById('logs-output');
            output.textContent = 'Connecting to logs...\n';

            if (currentLogController) {
                currentLogController.abort();
            }
            currentLogController = new AbortController();

            try {
                const response = await fetch('/jobs/' + encodeURIComponent(id) + '/logs', {
                    signal: currentLogController.signal
                });

                if (!response.ok) {
                    output.textContent += 'Error: ' + await response.text();
                    return;
                }

                const reader = response.body.getReader();
                const decoder = new TextDecoder();

                output.textContent = ''; // clear

                while (true) {
                    const { value, done } = await reader.read();
                    if (value) {
                        output.textContent += decoder.decode(value, { stream: true });
                        output.scrollTop = output.scrollHeight;
                    }
                    if (done) {
                        output.textContent += '\n--- End of logs ---';
                        break;
                    }
                }
            } catch (e) {
                if (e.name === 'AbortError') {
                    output.textContent += '\n--- Stream closed ---';
                } else {
                    output.textContent += '\nError: ' + e;
                }
            }
        }

        function closeLogs() {
            document.getElementById('logsModal').style.display = 'none';
            if (currentLogController) {
                currentLogController.abort();
                currentLogController = null;
            }
        }

        async function viewGraph() {
            const modal = document.getElementById('graphModal');
            const graphDiv = document.getElementById('graphDiv');
            modal.style.display = 'block';
            graphDiv.innerHTML = 'Loading graph...';

            try {
                const res = await fetch('/jobs/export/graph?format=mermaid');
                if (!res.ok) {
                    graphDiv.innerHTML = '<span style="color:red">Failed to load graph: ' + await res.text() + '</span>';
                    return;
                }
                const graphText = await res.text();
                if (!graphText.trim()) {
                    graphDiv.innerHTML = '<span>No jobs to display</span>';
                    return;
                }

                // Render mermaid
                const { svg } = await mermaid.render('mermaidGraph', graphText);
                graphDiv.innerHTML = svg;
            } catch (err) {
                console.error(err);
                graphDiv.innerHTML = '<span style="color:red">Error rendering graph: ' + err.message + '</span>';
            }
        }

        function closeGraph() {
            document.getElementById('graphModal').style.display = 'none';
        }

        function openSearchLogsModal() {
            document.getElementById('searchLogsModal').style.display = 'block';
            document.getElementById('search-logs-query').focus();
        }

        function closeSearchLogsModal() {
            document.getElementById('searchLogsModal').style.display = 'none';
        }

        async function performSearchLogs() {
            const query = document.getElementById('search-logs-query').value.trim();
            const tag = document.getElementById('search-logs-tag').value.trim();
            const status = document.getElementById('search-logs-status').value;
            const resultsDiv = document.getElementById('search-logs-results');

            if (!query) {
                alert("Please enter a regex query.");
                return;
            }

            resultsDiv.style.display = 'block';
            resultsDiv.innerHTML = 'Searching...';

            try {
                let url = '/jobs/search/logs?q=' + encodeURIComponent(query);
                if (tag) url += '&tag=' + encodeURIComponent(tag);
                if (status) url += '&status=' + encodeURIComponent(status);

                const res = await fetch(url);

                if (!res.ok) {
                    const errorText = await res.text();
                    resultsDiv.innerHTML = '<span style="color:red">Search failed: ' + errorText + '</span>';
                    return;
                }

                const results = await res.json();

                if (!results || results.length === 0) {
                    resultsDiv.innerHTML = '<span style="color: #bbb;">No matching logs found.</span>';
                    return;
                }

                let html = '';
                results.forEach(job => {
                    html += '<div style="margin-bottom: 20px;">';
                    html += '<div style="color: #61dafb; font-weight: bold; font-size: 1.1em;">Job: ' + escapeHTML(job.job_id) + ' <span style="color: #aaa; font-size: 0.9em; font-weight: normal;">(' + escapeHTML(job.status) + ')</span></div>';
                    html += '<div style="color: #ccc; margin-bottom: 5px;">Summary: ' + escapeHTML(job.summary) + '</div>';
                    html += '<div style="background: #111; padding: 10px; border-radius: 3px; border-left: 3px solid #61dafb;">';

                    job.matches.forEach(match => {
                        html += '<div style="display: flex; margin-bottom: 4px;">';
                        html += '<span style="color: #888; margin-right: 15px; user-select: none;">' + match.line_number + ':</span>';
                        html += '<span style="color: #e6db74; white-space: pre-wrap; word-break: break-all;">' + escapeHTML(match.text.trim()) + '</span>';
                        html += '</div>';
                    });

                    html += '</div></div>';
                });

                resultsDiv.innerHTML = html;

            } catch (err) {
                console.error(err);
                resultsDiv.innerHTML = '<span style="color:red">Error performing search: ' + err.message + '</span>';
            }
        }

        // Setup SSE for real-time updates
        const setupSSE = () => {
            const evtSource = new EventSource('/events');

            evtSource.onmessage = function(event) {
                try {
                    const data = JSON.parse(event.data);
                    if (data && data.event && data.event !== "connected") {
                        // Dynamically refresh the UI when an event occurs
                        fetchStatus();
                        fetchJobs();
                        if (data.event === "job_completed" || data.event === "job_failed" || data.event === "job_canceled") {
                            fetchAnalytics();
                        }
                    }
                } catch (e) {
                    console.error("Error parsing SSE data", e);
                }
            };

            evtSource.onerror = function(err) {
                console.error("EventSource failed:", err);
                evtSource.close();
                // Attempt to reconnect after 5 seconds
                setTimeout(setupSSE, 5000);
            };
        };

        setupSSE();

        // Keep slow polling as a fallback
        setInterval(fetchStatus, 30000);
        setInterval(fetchAnalytics, 60000);
        setInterval(fetchJobs, 30000);
    </script>
</body>
</html>
`
