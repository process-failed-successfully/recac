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
        .metric .label { font-weight: bold; color: #555; }
        .metric .value { font-family: monospace; }
        table { width: 100%; border-collapse: collapse; margin-top: 10px; }
        th, td { text-align: left; padding: 8px; border-bottom: 1px solid #ddd; }
        th { background-color: #f8f8f8; }
        tbody tr:hover { background-color: #f1f1f1; transition: background-color 0.15s ease-in-out; }
        .status-Completed { color: #198754; font-weight: bold; }
        .status-Failed, .status-Error { color: #d32f2f; font-weight: bold; }
        .status-Running, .status-Active, .status-Spawning { color: #0d6efd; font-weight: bold; }
        .status-Pending, .status-Pending-Approval { color: #b45309; font-weight: bold; }
        .status-Canceled { color: #6c757d; font-weight: bold; }
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
        .form-group input[type="text"], .form-group input[type="number"], .form-group textarea { width: 100%; padding: 8px; border: 1px solid #ccc; border-radius: 4px; box-sizing: border-box; }
        .form-group input[type="text"]:focus-visible, .form-group input[type="number"]:focus-visible, .form-group input[type="checkbox"]:focus-visible, .form-group textarea:focus-visible { outline: 2px solid #007bff; outline-offset: 2px; }
        .form-group textarea { resize: vertical; height: 100px; }
        button.danger { background: #dc3545; }
        button.danger:hover { background: #a82330; }
        #jobs-container { overflow-x: auto; }
        .controls { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px;}
        select, input { padding: 6px; }
        select:focus-visible, input:focus-visible { outline: 2px solid #007bff; outline-offset: 2px; }
        #logs-output { background: #222; color: #ddd; padding: 15px; border-radius: 4px; font-family: monospace; white-space: pre-wrap; overflow-y: auto; height: 400px; margin: 0; }
        .modal-large { width: 90%; max-width: 1000px; }
        @keyframes spin { 0% { transform: rotate(0deg); } 100% { transform: rotate(360deg); } }
        .spinner { display: inline-block; width: 12px; height: 12px; border: 2px solid rgba(255,255,255,0.3); border-radius: 50%; border-top-color: #fff; animation: spin 1s ease-in-out infinite; margin-right: 5px; vertical-align: middle; }
        [tabindex="0"]:focus-visible { outline: 2px solid #007bff; outline-offset: 2px; }
        .shortcut-hint {
            display: inline-block;
            padding: 2px 6px;
            font-size: 11px;
            background: rgba(255, 255, 255, 0.2);
            border-radius: 3px;
            font-family: monospace;
            border: 1px solid rgba(255, 255, 255, 0.3);
            vertical-align: middle;
        }
        @media (max-width: 768px) {
            .shortcut-hint { display: none; }
        }

        .sr-only {
            position: absolute;
            width: 1px;
            height: 1px;
            padding: 0;
            margin: -1px;
            overflow: hidden;
            clip: rect(0, 0, 0, 0);
            white-space: nowrap;
            border: 0;
        }
        .category-badge {
            display: inline-flex;
            align-items: center;
            background-color: #e9ecef;
            color: #495057;
            padding: 2px 8px;
            border-radius: 12px;
            font-size: 0.85em;
            font-weight: 500;
            margin-left: 10px;
            vertical-align: middle;
            cursor: help;
            text-decoration: underline dotted;
            text-underline-offset: 2px;
        }
    </style>
</head>
<body>
    <header>
        <h1>Orchestrator Dashboard</h1>
        <div id="connection-status" aria-live="polite">Connecting...</div>
    </header>
    <div class="container">
        <div class="controls" style="margin-top: 20px;">
            <div class="actions" id="global-actions" style="margin-top: 0;">
                <!-- Buttons will be injected here if supported -->
            </div>
            <div>
                <button type="button" onclick="generateChangelog(this)" aria-label="Generate Changelog" style="background-color: #17a2b8; margin-right: 10px;">Generate Changelog</button>
                <button type="button" onclick="generatePostmortem(this)" aria-label="Generate Postmortem" style="background-color: #dc3545; margin-right: 10px;">Generate Postmortem</button>
                <button type="button" onclick="openAnalyzeFailuresModal()" aria-label="Analyze Failures" style="background-color: #dc3545; margin-right: 10px;">Analyze Failures</button>
                <button type="button" onclick="openAnalyzeDurationsModal()" aria-label="Analyze Durations" style="background-color: #6f42c1; margin-right: 10px;">Analyze Durations</button>
                <button type="button" onclick="openAnalyzeCostsModal()" aria-label="Analyze Costs" style="background-color: #28a745; margin-right: 10px;">Analyze Costs</button>
                <button type="button" onclick="openAnalyzeAnomaliesModal()" aria-label="Analyze Anomalies" style="background-color: #e83e8c; margin-right: 10px;">Analyze Anomalies</button>
                <button type="button" onclick="openAnalyzeAgentsModal()" aria-label="Analyze Agents" style="background-color: #17a2b8; margin-right: 10px;">Analyze Agents</button>
                <button type="button" onclick="openReliabilityModal()" aria-label="Analyze Reliability" style="background-color: #007bff; margin-right: 10px;">Analyze Reliability</button>
                <button type="button" onclick="openSearchLogsModal()" aria-label="Search Logs" style="background-color: #6c757d; margin-right: 10px;">Search Logs</button>
                <button type="button" aria-label="View Graph" onclick="viewGraph()" style="background-color: #6f42c1; margin-right: 10px;">View Graph</button>
                <button type="button" aria-label="View Timeline" onclick="viewTimeline()" style="background-color: #fd7e14; margin-right: 10px;">View Timeline</button>
                <button type="button" aria-label="Export Trace" onclick="exportTrace(this)" style="background-color: #6c757d; margin-right: 10px;">Export Trace</button>
                <button type="button" aria-label="Export Pipeline" onclick="exportPipeline(this)" style="background-color: #6c757d; margin-right: 10px;">Export Pipeline</button>
                <button type="button" aria-label="Submit Pipeline" onclick="document.getElementById('submitPipelineModal').style.display='block'; setTimeout(() => document.getElementById('pipeline-yaml').focus(), 10);" style="background-color: #17a2b8; margin-right: 10px;">+ Submit Pipeline</button>
                <button type="button" aria-label="Submit Job" onclick="document.getElementById('submitModal').style.display='block'; setTimeout(() => document.getElementById('job-summary').focus(), 10);" aria-keyshortcuts="s" style="background-color: #28a745;">+ Submit Job<kbd aria-hidden="true" class="shortcut-hint">S</kbd></button>
            </div>
        </div>

        <div id="submitPipelineModal" class="modal" role="dialog" aria-modal="true">
            <div class="modal-content">
                <button type="button" class="close" aria-label="Close modal" title="Close (Esc)" onclick="document.getElementById('submitPipelineModal').style.display='none'"><span aria-hidden="true">&times;</span></button>
                <h2>Submit Pipeline (YAML)</h2>
                <form onsubmit="submitPipeline(); return false;">
                    <div class="form-group">
                        <label for="pipeline-yaml">Pipeline Definition <span aria-hidden="true" style="color: #d32f2f;">*</span></label>
                        <textarea id="pipeline-yaml" placeholder="name: my-pipeline&#10;jobs:&#10;  ..." style="height: 300px; font-family: monospace;" required aria-required="true"></textarea>
                    </div>
                    <div style="display: flex; gap: 10px;">
                        <button type="button" aria-label="Dry Run Pipeline" id="btn-dry-run" onclick="dryRunPipeline()" style="background-color: #6c757d; flex: 1;">Dry Run</button>
                        <button type="submit" aria-label="Submit Pipeline YAML" id="btn-submit-pipeline" style="background-color: #17a2b8; flex: 1;">Submit Pipeline</button>
                    </div>
                </form>
                <div id="dry-run-results" tabindex="0" aria-live="polite" style="display: none; margin-top: 15px; padding: 10px; background: #f8f9fa; border: 1px solid #dee2e6; border-radius: 4px; max-height: 200px; overflow-y: auto;">
                    <h3 style="margin-top: 0; font-size: 1.1em;">Dry Run Results</h3>
                    <pre id="dry-run-output" style="margin: 0; font-size: 0.9em; white-space: pre-wrap;"></pre>
                </div>
            </div>
        </div>

        <div id="timelineModal" class="modal" role="dialog" aria-modal="true">
            <div class="modal-content modal-large" style="width: 95%; max-width: 1400px; height: 90vh; display: flex; flex-direction: column;">
                <button type="button" class="close" aria-label="Close modal" title="Close (Esc)" onclick="closeTimeline()"><span aria-hidden="true">&times;</span></button>
                <h2 style="margin-bottom: 0;">Execution Timeline</h2>
                <div id="timelineDiv" tabindex="0" aria-live="polite" style="flex: 1; overflow: auto; display: flex; justify-content: center; align-items: flex-start; background: #fff; border: 1px solid #ccc; border-radius: 4px; margin-top: 15px;">
                    Loading timeline...
                </div>
            </div>
        </div>

        <div id="submitModal" class="modal" role="dialog" aria-modal="true">
            <div class="modal-content">
                <button type="button" class="close" aria-label="Close modal" title="Close (Esc)" onclick="document.getElementById('submitModal').style.display='none'"><span aria-hidden="true">&times;</span></button>
                <h2>Submit Ad-hoc Job</h2>
                <form onsubmit="submitAdHocJob(); return false;">
                    <div class="form-group">
                        <label for="job-id">Job ID (Optional, auto-generated if empty)</label>
                        <input type="text" id="job-id" placeholder="e.g., MY-JOB-123">
                    </div>
                    <div class="form-group">
                        <label for="job-summary">Summary <span aria-hidden="true" style="color: #d32f2f;">*</span></label>
                        <input type="text" id="job-summary" placeholder="e.g., Fix login bug" required aria-required="true">
                    </div>
                    <div class="form-group">
                        <label for="job-repo">Repository URL <span aria-hidden="true" style="color: #d32f2f;">*</span></label>
                        <input type="text" id="job-repo" placeholder="e.g., https://github.com/org/repo" required aria-required="true">
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
                    <button type="submit" aria-label="Submit Ad-hoc Job" id="btn-submit-adhoc" style="background-color: #28a745; width: 100%;">Submit Job</button>
                </form>
            </div>
        </div>

        <div id="editDepsModal" class="modal" role="dialog" aria-modal="true">
            <div class="modal-content">
                <button type="button" class="close" aria-label="Close modal" title="Close (Esc)" onclick="document.getElementById('editDepsModal').style.display='none'"><span aria-hidden="true">&times;</span></button>
                <h2>Edit Dependencies for <span id="edit-deps-job-id-display"></span></h2>
                <form onsubmit="submitEditDeps(); return false;">
                    <input type="hidden" id="edit-deps-job-id">
                    <div class="form-group">
                        <label for="edit-deps-input">Job IDs (comma-separated)</label>
                        <textarea id="edit-deps-input" placeholder="JOB-1, JOB-2" style="width: 100%; height: 60px; margin-bottom: 10px;"></textarea>
                    </div>
                    <button type="submit" aria-label="Save Dependencies" id="btn-submit-deps" style="background-color: #007bff; width: 100%;">Save Dependencies</button>
                </form>
            </div>
        </div>

        <div id="envModal" class="modal" role="dialog" aria-modal="true">
            <div class="modal-content modal-large">
                <button type="button" class="close" aria-label="Close modal" title="Close (Esc)" onclick="document.getElementById('envModal').style.display='none'"><span aria-hidden="true">&times;</span></button>
                <h2>Environment Variables for <span id="env-job-id-display"></span></h2>
                <form id="env-form" onsubmit="submitEnvVars(); return false;">
                    <input type="hidden" id="env-job-id">
                    <div id="env-vars-container" tabindex="0" style="max-height: 400px; overflow-y: auto; margin-bottom: 15px;">
                        <!-- Env fields will be injected here -->
                    </div>
                    <button type="button" aria-label="Add new environment variable" onclick="addEnvField('', '')" style="background-color: #6c757d; margin-bottom: 15px;">+ Add Variable</button>
                    <button type="submit" aria-label="Save Environment Variables" id="btn-submit-env" style="background-color: #28a745; width: 100%;">Save Environment Variables</button>
                </form>
            </div>
        </div>

        <div id="logsModal" class="modal" role="dialog" aria-modal="true">
            <div class="modal-content modal-large">
                <button type="button" class="close" aria-label="Close modal" title="Close (Esc)" onclick="closeLogs()"><span aria-hidden="true">&times;</span></button>
                <h2 id="logs-title">Job Logs</h2>
                <pre id="logs-output" tabindex="0" aria-live="polite"></pre>
            </div>
        </div>

        <div id="explainModal" class="modal" role="dialog" aria-modal="true">
            <div class="modal-content modal-large">
                <button type="button" class="close" aria-label="Close modal" title="Close (Esc)" onclick="closeExplainModal()"><span aria-hidden="true">&times;</span></button>
                <h2 id="explain-title">Job Explanation</h2>
                <div id="explain-content" tabindex="0" aria-live="polite" style="white-space: pre-wrap; font-family: sans-serif; line-height: 1.5; color: #333; background: #fff; padding: 15px; border-radius: 4px; border: 1px solid #ddd; max-height: 60vh; overflow-y: auto;">
                    Loading explanation...
                </div>
            </div>
        </div>

        <div id="reportModal" class="modal" role="dialog" aria-modal="true">
            <div class="modal-content modal-large">
                <button type="button" class="close" aria-label="Close modal" title="Close (Esc)" onclick="closeReportModal()"><span aria-hidden="true">&times;</span></button>
                <h2 id="report-title" style="margin-bottom: 0;">Report</h2>
                <div id="report-content" tabindex="0" aria-live="polite" style="max-height: 500px; overflow-y: auto; background: #fff; border: 1px solid #ccc; border-radius: 4px; padding: 15px; margin-top: 15px; white-space: pre-wrap; font-family: monospace;">
                    Loading report...
                </div>
            </div>
        </div>

        <div id="analyzeFailuresModal" class="modal" role="dialog" aria-modal="true">
            <div class="modal-content modal-large">
                <button type="button" class="close" aria-label="Close modal" title="Close (Esc)" onclick="closeAnalyzeFailuresModal()"><span aria-hidden="true">&times;</span></button>
                <h2 style="margin-bottom: 0;">Analyze Failures</h2>
                <div id="analyze-failures-content" tabindex="0" aria-live="polite" style="max-height: 500px; overflow-y: auto; background: #fff; border: 1px solid #ccc; border-radius: 4px; padding: 15px; margin-top: 15px;">
                    Loading analysis...
                </div>
            </div>
        </div>

        <div id="analyzeDurationsModal" class="modal" role="dialog" aria-modal="true">
            <div class="modal-content modal-large">
                <button type="button" class="close" aria-label="Close modal" title="Close (Esc)" onclick="closeAnalyzeDurationsModal()"><span aria-hidden="true">&times;</span></button>
                <h2 style="margin-bottom: 0;">Analyze Durations</h2>
                <div id="analyze-durations-content" tabindex="0" aria-live="polite" style="max-height: 500px; overflow-y: auto; background: #fff; border: 1px solid #ccc; border-radius: 4px; padding: 15px; margin-top: 15px;">
                    Loading analysis...
                </div>
            </div>
        </div>

        <div id="analyzeAnomaliesModal" class="modal" role="dialog" aria-modal="true">
            <div class="modal-content modal-large">
                <button type="button" class="close" aria-label="Close modal" title="Close (Esc)" onclick="closeAnalyzeAnomaliesModal()"><span aria-hidden="true">&times;</span></button>
                <h2 style="margin-bottom: 0;">Analyze Anomalies</h2>
                <div id="analyze-anomalies-content" tabindex="0" aria-live="polite" style="max-height: 500px; overflow-y: auto; background: #fff; border: 1px solid #ccc; border-radius: 4px; padding: 15px; margin-top: 15px;">
                    Loading analysis...
                </div>
            </div>
        </div>

        <div id="analyzeCostsModal" class="modal" role="dialog" aria-modal="true">
            <div class="modal-content modal-large">
                <button type="button" class="close" aria-label="Close modal" title="Close (Esc)" onclick="closeAnalyzeCostsModal()"><span aria-hidden="true">&times;</span></button>
                <h2 style="margin-bottom: 0;">Analyze Costs</h2>
                <div id="analyze-costs-content" tabindex="0" aria-live="polite" style="max-height: 500px; overflow-y: auto; background: #fff; border: 1px solid #ccc; border-radius: 4px; padding: 15px; margin-top: 15px;">
                    Loading analysis...
                </div>
            </div>
        </div>

        <div id="analyzeAgentsModal" class="modal" role="dialog" aria-modal="true">
            <div class="modal-content modal-large">
                <button type="button" class="close" aria-label="Close modal" title="Close (Esc)" onclick="closeAnalyzeAgentsModal()"><span aria-hidden="true">&times;</span></button>
                <h2 style="margin-bottom: 0;">Analyze Agents</h2>
                <div id="analyze-agents-content" tabindex="0" aria-live="polite" style="max-height: 500px; overflow-y: auto; background: #fff; border: 1px solid #ccc; border-radius: 4px; padding: 15px; margin-top: 15px;">
                    Loading analysis...
                </div>
            </div>
        </div>

        <div id="reliabilityModal" class="modal" role="dialog" aria-modal="true">
            <div class="modal-content modal-large">
                <button type="button" class="close" aria-label="Close modal" title="Close (Esc)" onclick="closeReliabilityModal()"><span aria-hidden="true">&times;</span></button>
                <h2 style="margin-bottom: 0;">Pipeline Reliability Report</h2>
                <div id="reliability-content" tabindex="0" aria-live="polite" style="max-height: 500px; overflow-y: auto; background: #fff; border: 1px solid #ccc; border-radius: 4px; padding: 15px; margin-top: 15px;">
                    Loading analysis...
                </div>
            </div>
        </div>

        <div id="searchLogsModal" class="modal" role="dialog" aria-modal="true">
            <div class="modal-content modal-large">
                <button type="button" class="close" aria-label="Close modal" title="Close (Esc)" onclick="closeSearchLogsModal()"><span aria-hidden="true">&times;</span></button>
                <h2>Search Logs</h2>
                <form onsubmit="performSearchLogs(); return false;">
                    <div style="display: flex; gap: 10px; margin-bottom: 15px; align-items: flex-end;">
                        <div class="form-group" style="flex: 2; margin-bottom: 0;">
                            <label for="search-logs-query" style="font-weight: normal; font-size: 0.9em; margin-bottom: 4px; display: block;">Regex Query <span aria-hidden="true" style="color: #d32f2f;">*</span></label>
                            <input type="text" id="search-logs-query" placeholder="(e.g., panic, error)..." required aria-required="true">
                        </div>
                        <div class="form-group" style="flex: 1; margin-bottom: 0;">
                            <label for="search-logs-tag" style="font-weight: normal; font-size: 0.9em; margin-bottom: 4px; display: block;">Tag Filter</label>
                            <input type="text" id="search-logs-tag" placeholder="(optional)">
                        </div>
                        <div class="form-group" style="flex: 1; margin-bottom: 0;">
                            <label for="search-logs-status" style="font-weight: normal; font-size: 0.9em; margin-bottom: 4px; display: block;">Status Filter</label>
                            <select id="search-logs-status" style="width: 100%; border: 1px solid #ccc; border-radius: 4px; padding: 8px; box-sizing: border-box;">
                                <option value="">Any Status</option>
                                <option value="Completed">Completed</option>
                                <option value="Failed">Failed</option>
                                <option value="Running">Running</option>
                                <option value="Canceled">Canceled</option>
                            </select>
                        </div>
                        <div class="form-group" style="flex: 1; margin-bottom: 0;">
                            <label for="search-logs-context" style="font-weight: normal; font-size: 0.9em; margin-bottom: 4px; display: block;">Context Lines</label>
                            <input type="number" id="search-logs-context" placeholder="Lines" min="0" value="0">
                        </div>
                        <button type="submit" aria-label="Execute Search" id="btn-search-logs" style="background-color: #007bff; min-width: 100px; height: 35px; margin-bottom: 1px;">Search</button>
                    </div>
                </form>
                <div id="search-logs-results" tabindex="0" aria-live="polite" style="max-height: 500px; overflow-y: auto; background: #222; color: #ddd; padding: 15px; border-radius: 4px; font-family: monospace; display: none;">
                    <!-- Results will be injected here -->
                </div>
            </div>
        </div>

        <div id="graphModal" class="modal" role="dialog" aria-modal="true">
            <div class="modal-content modal-large" style="width: 95%; max-width: 1400px; height: 90vh; display: flex; flex-direction: column;">
                <button type="button" class="close" aria-label="Close modal" title="Close (Esc)" onclick="closeGraph()"><span aria-hidden="true">&times;</span></button>
                <h2 style="margin-bottom: 0;">Dependency Graph</h2>
                <div id="graphDiv" tabindex="0" aria-live="polite" style="flex: 1; overflow: auto; display: flex; justify-content: center; align-items: center; background: #fff; border: 1px solid #ccc; border-radius: 4px; margin-top: 15px;">
                    Loading graph...
                </div>
            </div>
        </div>

        <div class="grid">
            <div class="card" id="status-card">
                <h2>Status</h2>
                <div id="status-content" aria-live="polite">Loading...</div>
            </div>
            <div class="card" id="analytics-card">
                <h2>Analytics</h2>
                <div id="analytics-content" aria-live="polite">Loading...</div>
            </div>
        </div>

        <div class="card" style="margin-top: 20px;">
            <div class="controls" style="align-items: flex-end;">
                <h2 style="margin-bottom: 0; display: inline-flex; align-items: center;">Jobs <span class="sr-only">Total Tasks: </span><span id="jobs-count-badge" class="category-badge" title="Total Tasks: 0">0</span></h2>
                <form id="jobs-filter-form" style="display: flex; gap: 10px; align-items: flex-end; margin: 0;">
                    <div class="form-group" style="margin-bottom: 0;">
                        <label for="job-state-filter" style="font-weight: normal; font-size: 0.9em; margin-bottom: 4px; display: block;">State Filter</label>
                        <select id="job-state-filter" aria-label="Filter jobs by state" style="width: 100%; border: 1px solid #ccc; border-radius: 4px; padding: 6px; box-sizing: border-box;">
                            <option value="">Active Jobs</option>
                            <option value="completed">Completed Jobs</option>
                            <option value="all">All Jobs</option>
                        </select>
                    </div>
                    <div class="form-group" style="margin-bottom: 0;">
                        <label for="job-search" style="font-weight: normal; font-size: 0.9em; margin-bottom: 4px; display: block;">Search Jobs</label>
                        <input type="text" id="job-search" placeholder="ID or Summary (Press '/')..." aria-label="Search jobs" aria-keyshortcuts="/" style="padding: 6px; border: 1px solid #ccc; border-radius: 4px; box-sizing: border-box;">
                    </div>
                    <button type="submit" aria-label="Refresh jobs list" id="refresh-jobs" aria-controls="jobs-container status-content analytics-content" aria-keyshortcuts="r" title="Shortcut: 'r'" style="margin-bottom: 1px; height: 31px;">Refresh<kbd aria-hidden="true" class="shortcut-hint">R</kbd></button>
                </form>
            </div>
            <div id="jobs-container" tabindex="0">
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
            const date = new Date(ds);
            return '<time datetime="' + date.toISOString() + '">' + date.toLocaleString() + '</time>';
        };


        function copyJobId(btn, id) {
            if (btn.getAttribute('aria-disabled') === 'true') return;
            navigator.clipboard.writeText(id);
            const originalText = btn.innerText;
            btn.innerText = 'Copied!';
            btn.setAttribute('aria-disabled', 'true');
            btn.style.cursor = 'default';
            const announcer = document.getElementById('a11y-announcer');
            if (announcer) {
                announcer.innerText = 'Copied job ID to clipboard';
                setTimeout(() => announcer.innerText = '', 3000);
            }
            setTimeout(() => {
                btn.innerText = originalText;
                btn.removeAttribute('aria-disabled');
                btn.style.cursor = 'pointer';
            }, 1500);
        }

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

        const renderStatusBadge = (status) => {
            if (!status) return '';
            const safeStatus = escapeHTML(status);
            const lowerStatus = status.toLowerCase();
            let icon = '';

            if (lowerStatus === 'completed') {
                icon = '✅';
            } else if (lowerStatus === 'failed' || lowerStatus === 'error') {
                icon = '❌';
            } else if (lowerStatus === 'running' || lowerStatus === 'active' || lowerStatus === 'spawning') {
                icon = '🔄';
            } else if (lowerStatus === 'pending' || lowerStatus === 'pending approval') {
                icon = '⏳';
            } else if (lowerStatus === 'canceled') {
                icon = '⏹️';
            }

            const iconHtml = icon ? '<span aria-hidden="true">' + icon + '</span>' : '';
            const srText = '<span class="sr-only">Status: </span>';
            const statusClass = 'status-' + safeStatus.replace(/\s+/g, '-');
            return '<span class="' + statusClass + '" title="Status: ' + safeStatus + '" style="display: inline-flex; align-items: center; gap: 4px; cursor: help; text-decoration: underline dotted; text-underline-offset: 2px;">' + srText + iconHtml + safeStatus + '</span>';
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
                    '<div class="metric"><span class="label">Circuit Broken:</span> <span class="value" style="color: ' + (data.circuit_broken ? '#d32f2f' : '#198754') + '">' + (data.circuit_broken ? 'True' : 'False') + '</span></div>' +
                    '<div class="metric"><span class="label">Max Concurrent:</span> <span class="value">' + (data.max_concurrent_jobs || 'Unlimited') + '</span></div>' +
                    '<div class="metric"><span class="label">State:</span> <span class="value" style="color: ' + (data.paused ? '#d32f2f' : '#198754') + '">' + (data.paused ? 'PAUSED' : 'RUNNING') + ' ' + (data.draining ? '(DRAINING)' : '') + '</span></div>';

                let actionsHTML = '';
                if(data.paused) {
                    actionsHTML += '<button type="button" aria-label="Resume polling" onclick="postAction(this, \'/resume\')">Resume</button>';
                } else {
                    actionsHTML += '<button type="button" aria-label="Pause polling" onclick="postAction(this, \'/pause\')">Pause</button>';
                }

                if(data.draining) {
                    actionsHTML += '<button type="button" aria-label="Undrain agents" onclick="postAction(this, \'/undrain\')">Undrain</button>';
                } else {
                    actionsHTML += '<button type="button" aria-label="Drain agents" class="danger" onclick="postAction(this, \'/drain\')">Drain</button>';
                }

                actionsHTML += '<button type="button" aria-label="Force manual poll" onclick="postAction(this, \'/poll\')">Force Poll</button>';
                actionsHTML += '<button type="button" aria-label="Clear all pending jobs" class="danger" onclick="deleteAction(this, \'/pending\')">Clear Pending</button>';
                actionsHTML += '<button type="button" aria-label="Clear all history jobs" class="danger" onclick="deleteAction(this, \'/history\')">Clear History</button>';
                actionsHTML += '<button type="button" aria-label="Retry all failed jobs" onclick="retryFailedJobs(this)" style="background-color: #ffc107; color: #212529;">Retry Failed</button>';

                document.getElementById('global-actions').innerHTML = actionsHTML;
                document.getElementById('connection-status').innerText = 'Connected';
                document.getElementById('connection-status').style.color = '#198754';
            } catch (err) {
                console.error('Error fetching status:', err);
                document.getElementById('connection-status').innerText = 'Disconnected';
                document.getElementById('connection-status').style.color = '#d32f2f';
            }
        }

        async function fetchAnalytics() {
            try {
                const res = await fetch('/analytics');
                const data = await res.json();
                let html = '<div class="metric"><span class="label">Total Jobs:</span> <span class="value">' + data.total_jobs + '</span></div>' +
                    '<div class="metric"><span class="label">Successful:</span> <span class="value" style="color: #198754">' + data.successful_jobs + '</span></div>' +
                    '<div class="metric"><span class="label">Failed:</span> <span class="value" style="color: #d32f2f">' + data.failed_jobs + '</span></div>' +
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

        async function retryFailedJobs(btn) {
            const originalText = btn.innerText;
            if(!confirm('Are you sure you want to retry all failed jobs?')) return;
            btn.disabled = true;
            btn.innerHTML = '<span class="spinner" aria-hidden="true"></span> Wait...';
            try {
                const res = await fetch('/jobs/retry-failed', { method: 'POST' });
                if(res.ok) {
                    setTimeout(() => {
                        btn.disabled = false;
                        btn.innerText = originalText;
                        fetchStatus();
                        fetchJobs();
                    }, 1000);
                } else {
                    alert('Action failed: ' + await res.text());
                    btn.disabled = false;
                    btn.innerText = originalText;
                }
            } catch(e) {
                alert('Request error: ' + e);
                btn.disabled = false;
                btn.innerText = originalText;
            }
        }

        async function doJobAction(btn, action, id) {
            if ((action === 'purge' || action === 'cancel' || action === 'skip') && !confirm('Are you sure you want to ' + action + ' job ' + id + '?')) {
                return;
            }
            const originalText = btn.innerText;
            btn.disabled = true;
            btn.innerHTML = '<span class="spinner" aria-hidden="true"></span> Wait...';
            let method = 'POST';
            let url = '';

            if (action === 'approve') {
                url = '/jobs/' + encodeURIComponent(id) + '/approve';
            } else if (action === 'skip') {
                url = '/jobs/' + encodeURIComponent(id) + '/skip';
            } else if (action === 'hold') {
                url = '/jobs/' + encodeURIComponent(id) + '/hold';
            } else if (action === 'unhold') {
                url = '/jobs/' + encodeURIComponent(id) + '/unhold';
            } else if (action === 'demote') {
                url = '/jobs/' + encodeURIComponent(id) + '/demote';
            } else if (action === 'promote') {
                url = '/jobs/' + encodeURIComponent(id) + '/promote';
            } else if (action === 'retry') {
                url = '/jobs/' + encodeURIComponent(id) + '/retry';
            } else if (action === 'heal') {
                url = '/jobs/' + encodeURIComponent(id) + '/heal';
            } else if (action === 'cancel') {
                if(!confirm('Are you sure you want to cancel job ' + id + '?')) {
                    btn.disabled = false;
                    btn.innerText = originalText;
                    return;
                }
                url = '/jobs/' + encodeURIComponent(id);
                method = 'DELETE';
            } else if (action === 'purge') {
                if(!confirm('Are you sure you want to purge job ' + id + '?')) {
                    btn.disabled = false;
                    btn.innerText = originalText;
                    return;
                }
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
            } finally {
                btn.disabled = false;
                btn.innerText = originalText;
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
                setTimeout(() => document.getElementById('job-summary').focus(), 10);
            } catch (e) {
                console.error("Error cloning job:", e);
                alert("Failed to clone job details.");
            }
        }

        function editEnvVars(encodedJobJson) {
            try {
                const j = JSON.parse(decodeURIComponent(encodedJobJson));
                document.getElementById("env-job-id").value = j.id;
                document.getElementById("env-job-id-display").innerText = j.id;
                const container = document.getElementById("env-vars-container");
                container.innerHTML = "";

                if (j.work_item && j.work_item.env_vars && Object.keys(j.work_item.env_vars).length > 0) {
                    for (const [key, val] of Object.entries(j.work_item.env_vars)) {
                        addEnvField(key, val);
                    }
                } else {
                    addEnvField("", "");
                }

                document.getElementById("envModal").style.display = "block";
                setTimeout(() => { const firstKey = document.getElementById('env-vars-container').querySelector('.env-key'); if (firstKey) firstKey.focus(); }, 10);
            } catch (e) {
                console.error("Error editing env vars:", e);
                alert("Failed to load job details.");
            }
        }

        let envFieldCounter = 0;

        function addEnvField(key, val) {
            const container = document.getElementById("env-vars-container");
            const div = document.createElement("div");
            div.style.display = "flex";
            div.style.gap = "10px";
            div.style.marginBottom = "10px";
            div.style.alignItems = "center";

            envFieldCounter++;
            const keyId = "env-key-" + envFieldCounter;
            const valId = "env-val-" + envFieldCounter;

            const keyLabel = document.createElement("label");
            keyLabel.htmlFor = keyId;
            keyLabel.innerText = "Key";
            keyLabel.style.marginRight = "5px";

            const keyInput = document.createElement("input");
            keyInput.type = "text";
            keyInput.id = keyId;
            keyInput.className = "env-key";
            keyInput.placeholder = "KEY";
            keyInput.value = key;
            keyInput.style.flex = "1";

            const valLabel = document.createElement("label");
            valLabel.htmlFor = valId;
            valLabel.innerText = "Value";
            valLabel.style.marginRight = "5px";

            const valInput = document.createElement("input");
            valInput.type = "text";
            valInput.id = valId;
            valInput.className = "env-val";
            valInput.placeholder = "VALUE";
            valInput.value = val;
            valInput.style.flex = "2";

            const removeBtn = document.createElement("button");
            removeBtn.type = "button";
            removeBtn.innerHTML = "<span aria-hidden=\"true\">&times;</span>";
            removeBtn.setAttribute("aria-label", "Remove environment variable");
            removeBtn.setAttribute("title", "Remove");
            removeBtn.className = "danger";
            removeBtn.onclick = function() {
                container.removeChild(div);
            };

            div.appendChild(keyLabel);
            div.appendChild(keyInput);
            div.appendChild(valLabel);
            div.appendChild(valInput);
            div.appendChild(removeBtn);
            container.appendChild(div);
        }

        async function submitEnvVars() {
            const btn = document.getElementById("btn-submit-env");
            btn.disabled = true;
            btn.innerHTML = "<span class=\"spinner\" aria-hidden=\"true\"></span> Saving...";

            const id = document.getElementById("env-job-id").value;
            const container = document.getElementById("env-vars-container");
            const envVars = {};

            const rows = container.children;
            for (let i = 0; i < rows.length; i++) {
                const row = rows[i];
                const keyInput = row.querySelector(".env-key");
                const valInput = row.querySelector(".env-val");

                if (keyInput && valInput) {
                    const key = keyInput.value.trim();
                    const val = valInput.value.trim();
                    if (key) {
                        envVars[key] = val;
                    }
                }
            }

            try {
                const res = await fetch("/jobs/" + encodeURIComponent(id) + "/env", {
                    method: "PUT",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ env_vars: envVars })
                });
                if (res.ok) {
                    document.getElementById("envModal").style.display = "none";
                    fetchStatus();
                    fetchJobs();
                } else {
                    alert("Failed to update environment variables: " + await res.text());
                }
            } catch(e) {
                alert("Request failed: " + e);
            } finally {
                btn.disabled = false;
                btn.innerText = "Save Environment Variables";
            }
        }

        function editDependencies(encodedJobJson) {
            try {
                const j = JSON.parse(decodeURIComponent(encodedJobJson));
                document.getElementById('edit-deps-job-id').value = j.id;
                document.getElementById('edit-deps-job-id-display').innerText = j.id;
                document.getElementById('edit-deps-input').value = (j.work_item.depends_on || []).join(', ');
                document.getElementById('editDepsModal').style.display = 'block';
                setTimeout(() => document.getElementById('edit-deps-input').focus(), 10);
            } catch (e) {
                console.error("Error editing dependencies:", e);
                alert("Failed to load job details.");
            }
        }

        async function submitEditDeps() {
            const btn = document.getElementById('btn-submit-deps');
            btn.disabled = true;
            btn.innerHTML = '<span class="spinner" aria-hidden="true"></span> Saving...';

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
            btn.innerHTML = '<span class="spinner" aria-hidden="true"></span> Running...';

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
            const btn = document.getElementById('btn-submit-pipeline');
            btn.disabled = true;
            btn.innerHTML = '<span class="spinner" aria-hidden="true"></span> Submitting...';

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

            const btn = document.getElementById('btn-submit-adhoc');
            btn.disabled = true;
            btn.innerHTML = '<span class="spinner" aria-hidden="true"></span> Submitting...';

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
                if (!state) { // Only update if we're on the default (active) view
                    document.title = '(' + jobs.length + ') Orchestrator Dashboard';
                }

                const countBadge = document.getElementById('jobs-count-badge');
                if (countBadge) {
                    countBadge.innerText = jobs.length;
                    countBadge.title = 'Total Tasks: ' + jobs.length;
                }

                tbody.innerHTML = '';

                if (jobs.length === 0) {
                    tbody.innerHTML = '<tr><td colspan="5" style="text-align: center; padding: 2em; color: #555;">No jobs found.<br><br><button type="button" aria-label="Submit a new job from empty state" onclick="document.getElementById(\'submitModal\').style.display=\'block\'; setTimeout(() => document.getElementById(\'job-summary\').focus(), 10);" style="background-color: #28a745;">+ Submit Job<kbd aria-hidden="true" class="shortcut-hint">S</kbd></button></td></tr>';
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
                        actionButtons += '<button type="button" aria-label="Approve job ' + escapeHTML(j.id) + '" style="margin-left:10px; padding:4px 8px; font-size:12px;" onclick="doJobAction(this, \'approve\', \'' + escapeHTML(j.id) + '\')">Approve</button>';
                    } else if (lowerStatus === 'pending') {
                        actionButtons += '<button type="button" aria-label="Skip job ' + escapeHTML(j.id) + '" style="margin-left:10px; padding:4px 8px; font-size:12px;" onclick="doJobAction(this, \'skip\', \'' + escapeHTML(j.id) + '\')">Skip</button>';
                        actionButtons += '<button type="button" aria-label="Demote job ' + escapeHTML(j.id) + '" style="margin-left:10px; padding:4px 8px; font-size:12px;" onclick="doJobAction(this, \'demote\', \'' + escapeHTML(j.id) + '\')">Demote</button>';
                        actionButtons += '<button type="button" aria-label="Promote job ' + escapeHTML(j.id) + '" style="margin-left:10px; padding:4px 8px; font-size:12px;" onclick="doJobAction(this, \'promote\', \'' + escapeHTML(j.id) + '\')">Promote</button>';
                        if (j.work_item && j.work_item.hold) {
                            actionButtons += '<button type="button" aria-label="Unhold job ' + escapeHTML(j.id) + '" style="margin-left:10px; padding:4px 8px; font-size:12px;" onclick="doJobAction(this, \'unhold\', \'' + escapeHTML(j.id) + '\')">Unhold</button>';
                        } else {
                            actionButtons += '<button type="button" aria-label="Hold job ' + escapeHTML(j.id) + '" style="margin-left:10px; padding:4px 8px; font-size:12px;" onclick="doJobAction(this, \'hold\', \'' + escapeHTML(j.id) + '\')">Hold</button>';
                        }
                    } else if (lowerStatus === 'failed') {
                        actionButtons += '<button type="button" aria-label="Retry job ' + escapeHTML(j.id) + '" style="margin-left:10px; padding:4px 8px; font-size:12px;" onclick="doJobAction(this, \'retry\', \'' + escapeHTML(j.id) + '\')">Retry</button>';
                        actionButtons += '<button type="button" aria-label="Heal job ' + escapeHTML(j.id) + '" style="margin-left:10px; padding:4px 8px; font-size:12px; background-color: #28a745;" onclick="doJobAction(this, \'heal\', \'' + escapeHTML(j.id) + '\')">Heal</button>';
                    }

                    if (lowerStatus === 'running' || lowerStatus === 'spawning' || lowerStatus === 'active' || lowerStatus === 'pending') {
                        actionButtons += '<button type="button" aria-label="Cancel job ' + escapeHTML(j.id) + '" class="danger" style="margin-left:10px; padding:4px 8px; font-size:12px;" onclick="doJobAction(this, \'cancel\', \'' + escapeHTML(j.id) + '\')">Cancel</button>';
                    }

                    if (lowerStatus === 'completed' || lowerStatus === 'failed' || lowerStatus === 'canceled' || lowerStatus === 'error') {
                        actionButtons += '<button type="button" aria-label="Purge job ' + escapeHTML(j.id) + '" class="danger" style="margin-left:10px; padding:4px 8px; font-size:12px;" onclick="doJobAction(this, \'purge\', \'' + escapeHTML(j.id) + '\')">Purge</button>';
                    }

                    if (lowerStatus === 'failed' || lowerStatus === 'error') {
                        actionButtons += '<button type="button" aria-label="Explain job ' + escapeHTML(j.id) + '" style="margin-left:10px; padding:4px 8px; font-size:12px; background-color: #17a2b8;" onclick="explainJob(\'' + escapeHTML(j.id) + '\')">Explain</button>';
                    }

                    actionButtons += '<button type="button" aria-label="View logs for job ' + escapeHTML(j.id) + '" style="margin-left:10px; padding:4px 8px; font-size:12px; background-color: #6c757d;" onclick="viewLogs(\'' + escapeHTML(j.id) + '\')">Logs</button>';
                    const safeJobJson = encodeURIComponent(JSON.stringify(j)).replace(/'/g, "%27");
                    if (lowerStatus === 'pending') {
                        actionButtons += '<button type="button" aria-label="Set dependencies for job ' + escapeHTML(j.id) + '" style="margin-left:10px; padding:4px 8px; font-size:12px; background-color: #ffc107; color: #212529;" onclick="editDependencies(\'' + safeJobJson + '\')">Set Deps</button>';
                    }
                    actionButtons += '<button type="button" aria-label="Edit Env Vars for job ' + escapeHTML(j.id) + '" style="margin-left:10px; padding:4px 8px; font-size:12px; background-color: #28a745;" onclick="editEnvVars(\'' + safeJobJson + '\')">Env Vars</button>';
                    actionButtons += '<button type="button" aria-label="Clone job ' + escapeHTML(j.id) + '" style="margin-left:10px; padding:4px 8px; font-size:12px; background-color: #17a2b8;" onclick="cloneJob(\'' + safeJobJson + '\')">Clone</button>';

                    let row = '<tr>' +
                        '<td><button type="button" aria-label="Copy job ID ' + safeId + '" title="Click to copy ID" style="background: none; border: none; padding: 0; color: #007bff; font-weight: bold; font-family: inherit; font-size: inherit; cursor: pointer; text-decoration: underline;" onclick="copyJobId(this, \'' + safeId + '\')">' + safeId + '</button></td>' +
                        '<td>' + safeSummary + '</td>' +
                        '<td>' + renderStatusBadge(j.status) + '</td>' +
                        '<td>' + formatDate(j.start_time) + '</td>' +
                        '<td>' + duration + actionButtons + '</td>' +
                    '</tr>';
                    tbody.innerHTML += row;
                });
            } catch (err) {
                console.error('Error fetching jobs:', err);
            }
        }

        async function postAction(btn, endpoint) {
            const originalText = btn.innerText;
            btn.disabled = true;
            btn.innerHTML = '<span class="spinner" aria-hidden="true"></span> Wait...';
            try {
                const res = await fetch(endpoint, { method: 'POST' });
                if(res.ok) {
                    fetchStatus();
                } else {
                    alert('Action failed: ' + await res.text());
                }
            } catch(e) {
                alert('Request failed: ' + e);
            } finally {
                btn.disabled = false;
                btn.innerText = originalText;
            }
        }

        async function deleteAction(btn, endpoint) {
            const originalText = btn.innerText;
            if(!confirm('Are you sure you want to perform this delete action?')) return;
            btn.disabled = true;
            btn.innerHTML = '<span class="spinner" aria-hidden="true"></span> Wait...';
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
            } finally {
                btn.disabled = false;
                btn.innerText = originalText;
            }
        }

        document.getElementById('jobs-filter-form').addEventListener('submit', async function(e) {
            e.preventDefault();
            const btn = document.getElementById('refresh-jobs');
            const originalText = btn.innerText;
            btn.disabled = true;
            btn.innerHTML = '<span class="spinner" aria-hidden="true"></span> Wait...';
            try {
                await fetchJobs();
            } finally {
                btn.disabled = false;
                btn.innerText = originalText;
            }
        });
        document.getElementById('job-state-filter').addEventListener('change', fetchJobs);

        // Init loops
        fetchStatus();
        fetchAnalytics();
        fetchJobs();

        let currentLogController = null;

        async function viewLogs(id) {
            document.getElementById('logsModal').style.display = 'block';
            setTimeout(() => document.getElementById('logs-output').focus(), 10);
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

        async function explainJob(id) {
            document.getElementById('explainModal').style.display = 'block';
            setTimeout(() => document.getElementById('explain-content').focus(), 10);
            document.getElementById('explain-title').innerText = 'Explanation for ' + id;
            const content = document.getElementById('explain-content');
            content.innerHTML = '<i>Asking AI to analyze the failure...</i>';

            try {
                const response = await fetch('/jobs/' + encodeURIComponent(id) + '/explain');
                if (!response.ok) {
                    content.innerText = 'Error fetching explanation: ' + await response.text();
                    return;
                }

                const data = await response.json();
                if (data.explanation && data.explanation.trim() !== '') {
                    content.innerText = data.explanation;
                } else {
                    content.innerHTML = '<div style="text-align: center; padding: 2em; color: #555;"><p>No explanation provided.</p><div style="margin-top: 15px;"><button type="button" onclick="this.closest(\'.modal\').style.display=\'none\'" style="background-color: #6c757d; color: white; padding: 6px 12px; border: none; border-radius: 4px; cursor: pointer;">Close (Esc)</button></div></div>';
                }
            } catch (err) {
                content.innerText = 'Error: ' + err.message;
            }
        }

        function closeExplainModal() {
            document.getElementById('explainModal').style.display = 'none';
        }

        async function viewGraph() {
            const modal = document.getElementById('graphModal');
            const graphDiv = document.getElementById('graphDiv');
            modal.style.display = 'block';
            setTimeout(() => graphDiv.focus(), 10);
            graphDiv.innerHTML = 'Loading graph...';

            try {
                const res = await fetch('/jobs/export/graph?format=mermaid');
                if (!res.ok) {
                    graphDiv.innerHTML = '<span style="color:#d32f2f">Failed to load graph: ' + await res.text() + '</span>';
                    return;
                }
                const graphText = await res.text();
                if (!graphText.trim()) {
                    graphDiv.innerHTML = '<div style="text-align: center; padding: 2em; color: #555;"><p>No jobs to display.</p><div style="margin-top: 15px;"><button type="button" onclick="this.closest(\'.modal\').style.display=\'none\'; document.getElementById(\'submitModal\').style.display=\'block\'; setTimeout(() => document.getElementById(\'job-summary\').focus(), 10);" style="background-color: #28a745; color: white; padding: 6px 12px; border: none; border-radius: 4px; cursor: pointer;">+ Submit Job<kbd aria-hidden="true" class="shortcut-hint">S</kbd></button></div></div>';
                    return;
                }

                // Render mermaid
                const { svg } = await mermaid.render('mermaidGraph', graphText);
                graphDiv.innerHTML = svg;
            } catch (err) {
                console.error(err);
                graphDiv.innerHTML = '<span style="color:#d32f2f">Error rendering graph: ' + err.message + '</span>';
            }
        }

        function closeGraph() {
            document.getElementById('graphModal').style.display = 'none';
        }

        async function viewTimeline() {
            const modal = document.getElementById('timelineModal');
            const timelineDiv = document.getElementById('timelineDiv');
            modal.style.display = 'block';
            setTimeout(() => timelineDiv.focus(), 10);
            timelineDiv.innerHTML = 'Loading timeline...';

            try {
                const res = await fetch('/jobs/export/timeline');
                if (!res.ok) {
                    timelineDiv.innerHTML = '<span style="color:#d32f2f">Failed to load timeline: ' + await res.text() + '</span>';
                    return;
                }
                const timelineText = await res.text();
                if (!timelineText.trim() || timelineText.trim() === "gantt\n    title Job Execution Timeline") {
                    timelineDiv.innerHTML = '<div style="text-align: center; padding: 2em; color: #555;"><p>No jobs to display.</p><div style="margin-top: 15px;"><button type="button" onclick="this.closest(\'.modal\').style.display=\'none\'; document.getElementById(\'submitModal\').style.display=\'block\'; setTimeout(() => document.getElementById(\'job-summary\').focus(), 10);" style="background-color: #28a745; color: white; padding: 6px 12px; border: none; border-radius: 4px; cursor: pointer;">+ Submit Job<kbd aria-hidden="true" class="shortcut-hint">S</kbd></button></div></div>';
                    return;
                }

                // Render mermaid
                const { svg } = await mermaid.render('mermaidTimeline', timelineText);
                timelineDiv.innerHTML = svg;
            } catch (err) {
                console.error(err);
                timelineDiv.innerHTML = '<span style="color:#d32f2f">Error rendering timeline: ' + err.message + '</span>';
            }
        }

        function closeTimeline() {
            document.getElementById('timelineModal').style.display = 'none';
        }

        async function exportTrace(btn) {
            const originalText = btn.innerText;
            btn.disabled = true;
            btn.innerHTML = '<span class="spinner" aria-hidden="true"></span> Exporting...';
            try {
                const res = await fetch('/jobs/export/trace');
                if (!res.ok) {
                    const text = await res.text();
                    alert('Failed to export trace: ' + text);
                    return;
                }
                const blob = await res.blob();
                const url = window.URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.style.display = 'none';
                a.href = url;
                a.download = 'trace.json';
                document.body.appendChild(a);
                a.click();
                window.URL.revokeObjectURL(url);
            } catch (err) {
                console.error('Error exporting trace:', err);
                alert('Error exporting trace: ' + err.message);
            } finally {
                btn.disabled = false;
                btn.innerText = originalText;
            }
        }

        async function exportPipeline(btn) {
            const originalText = btn.innerText;
            btn.disabled = true;
            btn.innerHTML = '<span class="spinner" aria-hidden="true"></span> Exporting...';
            try {
                const res = await fetch('/jobs/export/pipeline?name=dashboard-export');
                if (!res.ok) {
                    const text = await res.text();
                    alert('Failed to export pipeline: ' + text);
                    return;
                }
                const blob = await res.blob();
                const url = window.URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.style.display = 'none';
                a.href = url;
                a.download = 'dashboard-export.yaml';
                document.body.appendChild(a);
                a.click();
                window.URL.revokeObjectURL(url);
            } catch (err) {
                console.error('Error exporting pipeline:', err);
                alert('Error exporting pipeline: ' + err.message);
            } finally {
                btn.disabled = false;
                btn.innerText = originalText;
            }
        }

        async function openAnalyzeFailuresModal() {
            const modal = document.getElementById('analyzeFailuresModal');
            const contentDiv = document.getElementById('analyze-failures-content');
            modal.style.display = 'block';
            setTimeout(() => contentDiv.focus(), 10);
            contentDiv.innerHTML = 'Loading analysis...';

            try {
                const res = await fetch('/jobs?state=all&status=Failed');
                if (!res.ok) {
                    contentDiv.innerHTML = '<span style="color:#d32f2f">Failed to load jobs: ' + await res.text() + '</span>';
                    return;
                }
                const jobs = await res.json();
                if (!jobs || jobs.length === 0) {
                    contentDiv.innerHTML = '<div style="text-align: center; padding: 2em; color: #555;"><p>No failed jobs found.</p><div style="margin-top: 15px;"><button type="button" onclick="this.closest(\'.modal\').style.display=\'none\'" style="background-color: #6c757d; color: white; padding: 6px 12px; border: none; border-radius: 4px; cursor: pointer;">Go Back (Esc)</button></div></div>';
                    return;
                }

                // Group jobs by summary
                const summaryMap = {};
                jobs.forEach(job => {
                    let summary = (job.summary || '').trim();
                    if (!summary) {
                        summary = '<empty summary>';
                    }
                    if (!summaryMap[summary]) {
                        summaryMap[summary] = [];
                    }
                    summaryMap[summary].push(job.id);
                });

                // Convert to array and sort by count descending, then alphabetical
                const groups = Object.keys(summaryMap).map(summary => {
                    return { summary: summary, jobIDs: summaryMap[summary], count: summaryMap[summary].length };
                });
                groups.sort((a, b) => {
                    if (a.count !== b.count) {
                        return b.count - a.count;
                    }
                    return a.summary.localeCompare(b.summary);
                });

                let html = '<p><strong>Total Failed Jobs:</strong> ' + jobs.length + '</p>';
                html += '<table style="width: 100%; border-collapse: collapse;">';
                html += '<thead><tr><th style="width: 10%;">Count</th><th style="width: 50%;">Error Signature (Summary)</th><th style="width: 40%;">Job IDs</th></tr></thead><tbody>';

                groups.forEach(g => {
                    let displayIDs = g.jobIDs.join(', ');
                    if (displayIDs.length > 50) {
                        displayIDs = displayIDs.substring(0, 47) + '...';
                    }

                    html += '<tr>';
                    html += '<td style="vertical-align: top;">' + g.count + '</td>';
                    html += '<td style="vertical-align: top;">' + escapeHTML(g.summary) + '</td>';
                    html += '<td style="vertical-align: top;"><span title="' + escapeHTML(g.jobIDs.join(', ')) + '">' + escapeHTML(displayIDs) + '</span></td>';
                    html += '</tr>';
                });

                html += '</tbody></table>';
                contentDiv.innerHTML = html;
            } catch (err) {
                console.error(err);
                contentDiv.innerHTML = '<span style="color:#d32f2f">Error rendering analysis: ' + err.message + '</span>';
            }
        }

        async function generateChangelog(btn) {
            const originalText = btn.innerText;
            btn.disabled = true;
            btn.innerHTML = '<span class="spinner" aria-hidden="true"></span> Generating...';

            const modal = document.getElementById('reportModal');
            const titleElement = document.getElementById('report-title');
            const contentDiv = document.getElementById('report-content');

            modal.style.display = 'block';
            setTimeout(() => contentDiv.focus(), 10);
            titleElement.innerText = 'Changelog Report';
            contentDiv.innerHTML = 'Generating AI changelog report...';

            try {
                const res = await fetch('/changelog/generate');
                if (!res.ok) {
                    contentDiv.innerHTML = '<span style="color:#d32f2f">Failed to generate changelog: ' + await res.text() + '</span>';
                    return;
                }
                const data = await res.json();
                if (data.changelog && data.changelog.trim() !== '') {
                    contentDiv.innerText = data.changelog;
                } else {
                    contentDiv.innerHTML = '<div style="text-align: center; padding: 2em; color: #555;"><p>No changelog generated.</p><div style="margin-top: 15px;"><button type="button" onclick="this.closest(\'.modal\').style.display=\'none\'" style="background-color: #6c757d; color: white; padding: 6px 12px; border: none; border-radius: 4px; cursor: pointer;">Close (Esc)</button></div></div>';
                }
            } catch (err) {
                console.error(err);
                contentDiv.innerHTML = '<span style="color:#d32f2f">Error generating changelog: ' + err.message + '</span>';
            } finally {
                btn.disabled = false;
                btn.innerText = originalText;
            }
        }

        async function generatePostmortem(btn) {
            const originalText = btn.innerText;
            btn.disabled = true;
            btn.innerHTML = '<span class="spinner" aria-hidden="true"></span> Generating...';

            const modal = document.getElementById('reportModal');
            const titleElement = document.getElementById('report-title');
            const contentDiv = document.getElementById('report-content');

            modal.style.display = 'block';
            setTimeout(() => contentDiv.focus(), 10);
            titleElement.innerText = 'Postmortem Report';
            contentDiv.innerHTML = 'Generating AI postmortem report...';

            try {
                const res = await fetch('/postmortem/generate');
                if (!res.ok) {
                    contentDiv.innerHTML = '<span style="color:#d32f2f">Failed to generate postmortem: ' + await res.text() + '</span>';
                    return;
                }
                const data = await res.json();
                if (data.postmortem && data.postmortem.trim() !== '') {
                    contentDiv.innerText = data.postmortem;
                } else {
                    contentDiv.innerHTML = '<div style="text-align: center; padding: 2em; color: #555;"><p>No postmortem generated.</p><div style="margin-top: 15px;"><button type="button" onclick="this.closest(\'.modal\').style.display=\'none\'" style="background-color: #6c757d; color: white; padding: 6px 12px; border: none; border-radius: 4px; cursor: pointer;">Close (Esc)</button></div></div>';
                }
            } catch (err) {
                console.error(err);
                contentDiv.innerHTML = '<span style="color:#d32f2f">Error generating postmortem: ' + err.message + '</span>';
            } finally {
                btn.disabled = false;
                btn.innerText = originalText;
            }
        }

        function closeReportModal() {
            document.getElementById('reportModal').style.display = 'none';
        }

        function closeAnalyzeFailuresModal() {
            document.getElementById('analyzeFailuresModal').style.display = 'none';
        }

        async function openAnalyzeDurationsModal() {
            const modal = document.getElementById('analyzeDurationsModal');
            const contentDiv = document.getElementById('analyze-durations-content');
            modal.style.display = 'block';
            setTimeout(() => contentDiv.focus(), 10);
            contentDiv.innerHTML = 'Loading analysis...';

            try {
                const res = await fetch('/jobs/analyze/durations?limit=10');
                if (!res.ok) {
                    contentDiv.innerHTML = '<span style="color:#d32f2f">Failed to load duration analysis: ' + await res.text() + '</span>';
                    return;
                }
                const data = await res.json();
                if (!data || data.total_jobs === 0) {
                    contentDiv.innerHTML = '<div style="text-align: center; padding: 2em; color: #555;"><p>No valid completed jobs with duration found.</p><div style="margin-top: 15px;"><button type="button" onclick="this.closest(\'.modal\').style.display=\'none\'" style="background-color: #6c757d; color: white; padding: 6px 12px; border: none; border-radius: 4px; cursor: pointer;">Go Back (Esc)</button></div></div>';
                    return;
                }

                let html = '<h3>Overall Statistics</h3>';
                html += '<div style="display: flex; gap: 20px; flex-wrap: wrap;">';
                html += '<div><strong>Total Jobs:</strong> ' + data.total_jobs + '</div>';
                html += '<div><strong>Mean:</strong> ' + (data.mean_duration_ms / 1000).toFixed(2) + 's</div>';
                html += '<div><strong>Median:</strong> ' + (data.median_duration_ms / 1000).toFixed(2) + 's</div>';
                html += '<div><strong>Min:</strong> ' + (data.min_duration_ms / 1000).toFixed(2) + 's</div>';
                html += '<div><strong>Max:</strong> ' + (data.max_duration_ms / 1000).toFixed(2) + 's</div>';
                html += '<div><strong>Total:</strong> ' + (data.total_duration_ms / 1000).toFixed(2) + 's</div>';
                html += '</div>';

                if (data.tag_stats && data.tag_stats.length > 0) {
                    html += '<h3 style="margin-top: 20px;">Average Duration by Tag</h3>';
                    html += '<table><thead><tr><th>Tag</th><th>Count</th><th>Mean Duration (s)</th></tr></thead><tbody>';
                    data.tag_stats.forEach(ts => {
                        html += '<tr><td>' + escapeHTML(ts.tag) + '</td><td>' + ts.count + '</td><td>' + (ts.mean_duration_ms / 1000).toFixed(2) + '</td></tr>';
                    });
                    html += '</tbody></table>';
                }

                if (data.top_slowest && data.top_slowest.length > 0) {
                    html += '<h3 style="margin-top: 20px;">Top ' + data.top_slowest.length + ' Slowest Jobs</h3>';
                    html += '<table><thead><tr><th>ID</th><th>Summary</th><th>Status</th><th>Duration (s)</th></tr></thead><tbody>';
                    data.top_slowest.forEach(job => {
                        const start = new Date(job.start_time).getTime();
                        const end = new Date(job.end_time).getTime();
                        const duration = ((end - start) / 1000).toFixed(2);
                        html += '<tr><td>' + escapeHTML(job.id) + '</td><td>' + escapeHTML(job.summary) + '</td><td>' + renderStatusBadge(job.status) + '</td><td>' + duration + '</td></tr>';
                    });
                    html += '</tbody></table>';
                }

                contentDiv.innerHTML = html;
            } catch (err) {
                console.error(err);
                contentDiv.innerHTML = '<span style="color:#d32f2f">Error fetching duration analysis: ' + err.message + '</span>';
            }
        }

        function closeAnalyzeDurationsModal() {
            document.getElementById('analyzeDurationsModal').style.display = 'none';
        }

        async function openAnalyzeCostsModal() {
            const modal = document.getElementById('analyzeCostsModal');
            const contentDiv = document.getElementById('analyze-costs-content');
            modal.style.display = 'block';
            setTimeout(() => contentDiv.focus(), 10);
            contentDiv.innerHTML = 'Loading analysis...';

            try {
                const res = await fetch('/jobs/analyze/costs?limit=10');
                if (!res.ok) {
                    contentDiv.innerHTML = '<span style="color:#d32f2f">Failed to load cost analysis: ' + await res.text() + '</span>';
                    return;
                }
                const data = await res.json();
                if (!data || data.total_stats.total_jobs === 0) {
                    contentDiv.innerHTML = '<div style="text-align: center; padding: 2em; color: #555;"><p>No valid completed jobs with cost data found.</p><div style="margin-top: 15px;"><button type="button" onclick="this.closest(\'.modal\').style.display=\'none\'" style="background-color: #6c757d; color: white; padding: 6px 12px; border: none; border-radius: 4px; cursor: pointer;">Go Back (Esc)</button></div></div>';
                    return;
                }

                let html = '<h3>Overall Statistics</h3>';
                html += '<div style="display: flex; gap: 20px; flex-wrap: wrap;">';
                html += '<div><strong>Total Jobs:</strong> ' + data.total_stats.total_jobs + '</div>';
                html += '<div><strong>Total Cost:</strong> $' + data.total_stats.total_cost.toFixed(4) + '</div>';
                html += '<div><strong>Total Tokens (Prompt):</strong> ' + data.total_stats.total_tokens_prompt + '</div>';
                html += '<div><strong>Total Tokens (Completion):</strong> ' + data.total_stats.total_tokens_completion + '</div>';
                html += '</div>';

                if (data.model_stats && data.model_stats.length > 0) {
                    html += '<h3 style="margin-top: 20px;">Cost by Model</h3>';
                    html += '<table><thead><tr><th>Model</th><th>Count</th><th>Cost ($)</th><th>Tokens (Prompt)</th><th>Tokens (Completion)</th></tr></thead><tbody>';
                    data.model_stats.forEach(ms => {
                        html += '<tr><td>' + escapeHTML(ms.model) + '</td><td>' + ms.jobs_count + '</td><td>' + ms.cost.toFixed(4) + '</td><td>' + ms.tokens_prompt + '</td><td>' + ms.tokens_completion + '</td></tr>';
                    });
                    html += '</tbody></table>';
                }

                if (data.tag_stats && data.tag_stats.length > 0) {
                    html += '<h3 style="margin-top: 20px;">Cost by Tag</h3>';
                    html += '<table><thead><tr><th>Tag</th><th>Count</th><th>Cost ($)</th><th>Tokens (Prompt)</th><th>Tokens (Completion)</th></tr></thead><tbody>';
                    data.tag_stats.forEach(ts => {
                        html += '<tr><td>' + escapeHTML(ts.tag) + '</td><td>' + ts.jobs_count + '</td><td>' + ts.cost.toFixed(4) + '</td><td>' + ts.tokens_prompt + '</td><td>' + ts.tokens_completion + '</td></tr>';
                    });
                    html += '</tbody></table>';
                }

                if (data.top_expensive_jobs && data.top_expensive_jobs.length > 0) {
                    html += '<h3 style="margin-top: 20px;">Top ' + data.top_expensive_jobs.length + ' Expensive Jobs</h3>';
                    html += '<table><thead><tr><th>ID</th><th>Summary</th><th>Model</th><th>Cost ($)</th></tr></thead><tbody>';
                    data.top_expensive_jobs.forEach(job => {
                        const cost = job.metrics && job.metrics.cost_usd ? job.metrics.cost_usd.toFixed(4) : "0.0000";
                        const model = job.agent_model || "unknown";
                        html += '<tr><td>' + escapeHTML(job.id) + '</td><td>' + escapeHTML(job.summary) + '</td><td>' + escapeHTML(model) + '</td><td>' + cost + '</td></tr>';
                    });
                    html += '</tbody></table>';
                }

                contentDiv.innerHTML = html;
            } catch (err) {
                console.error(err);
                contentDiv.innerHTML = '<span style="color:#d32f2f">Error fetching cost analysis: ' + err.message + '</span>';
            }
        }

        function closeAnalyzeCostsModal() {
            document.getElementById('analyzeCostsModal').style.display = 'none';
        }

        async function openAnalyzeAnomaliesModal() {
            const modal = document.getElementById('analyzeAnomaliesModal');
            const contentDiv = document.getElementById('analyze-anomalies-content');
            modal.style.display = 'block';
            setTimeout(() => contentDiv.focus(), 10);
            contentDiv.innerHTML = 'Loading analysis...';

            try {
                const res = await fetch('/jobs/analyze/anomalies');
                if (!res.ok) {
                    contentDiv.innerHTML = '<span style="color:#d32f2f">Failed to load anomalies analysis: ' + await res.text() + '</span>';
                    return;
                }
                const anomalies = await res.json();
                if (!anomalies || anomalies.length === 0) {
                    contentDiv.innerHTML = '<div style="text-align: center; padding: 2em; color: #555;"><p>No anomalies found.</p><div style="margin-top: 15px;"><button type="button" onclick="this.closest(\'.modal\').style.display=\'none\'" style="background-color: #6c757d; color: white; padding: 6px 12px; border: none; border-radius: 4px; cursor: pointer;">Go Back (Esc)</button></div></div>';
                    return;
                }

                // Sort anomalies by most deviant duration first
                anomalies.sort((a, b) => b.duration_dev - a.duration_dev);

                let html = '<p><strong>Top Anomalies:</strong> ' + anomalies.length + '</p>';
                html += '<table style="width: 100%; border-collapse: collapse;">';
                html += '<thead><tr><th>Job ID</th><th>Model</th><th>Status</th><th>Duration</th><th>Cost</th><th>Dur Dev</th><th>Cost Dev</th></tr></thead><tbody>';

                anomalies.forEach(a => {
                    let durDevStr = a.duration_dev ? a.duration_dev.toFixed(2) + 'σ' : '-';
                    let costDevStr = a.cost_dev ? a.cost_dev.toFixed(2) + 'σ' : '-';

                    let durationStr = a.duration ? a.duration + 'ns' : '-';
                    // Very simple formatter. If duration comes as nanoseconds, convert it manually or let the backend send string.
                    // Wait, the API returns time.Duration which marshals to nanoseconds integer in JSON. Let's fix that.
                    // Let's divide by 1e9 to get seconds.
                    let durationSeconds = (a.duration / 1000000000).toFixed(2) + 's';

                    html += '<tr>' +
                        '<td>' + escapeHTML(a.job_id) + '</td>' +
                        '<td>' + escapeHTML(a.model) + '</td>' +
                        '<td>' + renderStatusBadge(a.status) + '</td>' +
                        '<td>' + durationSeconds + '</td>' +
                        '<td>$' + (a.cost || 0).toFixed(4) + '</td>' +
                        '<td>' + durDevStr + '</td>' +
                        '<td>' + costDevStr + '</td>' +
                        '</tr>';
                });
                html += '</tbody></table>';

                contentDiv.innerHTML = html;

            } catch (err) {
                console.error(err);
                contentDiv.innerHTML = '<span style="color:#d32f2f">Error fetching anomalies analysis: ' + err.message + '</span>';
            }
        }

        function closeAnalyzeAnomaliesModal() {
            document.getElementById('analyzeAnomaliesModal').style.display = 'none';
        }

        async function openAnalyzeAgentsModal() {
            const modal = document.getElementById('analyzeAgentsModal');
            const contentDiv = document.getElementById('analyze-agents-content');
            modal.style.display = 'block';
            setTimeout(() => contentDiv.focus(), 10);
            contentDiv.innerHTML = 'Loading analysis...';

            try {
                const res = await fetch('/jobs/analyze/agents?limit=10');
                if (!res.ok) {
                    contentDiv.innerHTML = '<span style="color:#d32f2f">Failed to load agent analysis: ' + await res.text() + '</span>';
                    return;
                }
                const data = await res.json();
                if (!data.agents || data.agents.length === 0) {
                    contentDiv.innerHTML = '<div style="text-align: center; padding: 2em; color: #555;"><p>No agent data found.</p><div style="margin-top: 15px;"><button type="button" onclick="this.closest(\'.modal\').style.display=\'none\'" style="background-color: #6c757d; color: white; padding: 6px 12px; border: none; border-radius: 4px; cursor: pointer;">Go Back (Esc)</button></div></div>';
                    return;
                }

                let html = '<p><strong>Top Agents:</strong> ' + data.agents.length + '</p>';
                html += '<table style="width: 100%; border-collapse: collapse;">';
                html += '<thead><tr><th>Provider</th><th>Model</th><th>Total Jobs</th><th>Success %</th><th>Avg Duration</th><th>Avg Cost</th><th>Total Cost</th></tr></thead><tbody>';

                data.agents.forEach(a => {
                    let successPct = (a.success_rate * 100).toFixed(1) + '%';
                    let avgDur = (a.average_duration / 1000000000).toFixed(2) + 's';
                    html += '<tr>' +
                        '<td>' + escapeHTML(a.agent_provider) + '</td>' +
                        '<td>' + escapeHTML(a.agent_model) + '</td>' +
                        '<td>' + a.total_jobs + ' (' + a.successful_jobs + ' ok / ' + a.failed_jobs + ' fail)</td>' +
                        '<td>' + successPct + '</td>' +
                        '<td>' + avgDur + '</td>' +
                        '<td>$' + (a.average_cost || 0).toFixed(4) + '</td>' +
                        '<td>$' + (a.total_cost || 0).toFixed(4) + '</td>' +
                        '</tr>';
                });
                html += '</tbody></table>';

                contentDiv.innerHTML = html;

            } catch (err) {
                console.error(err);
                contentDiv.innerHTML = '<span style="color:#d32f2f">Error fetching agent analysis: ' + err.message + '</span>';
            }
        }

        function closeAnalyzeAgentsModal() {
            document.getElementById('analyzeAgentsModal').style.display = 'none';
        }

        function openReliabilityModal() {
            const modal = document.getElementById('reliabilityModal');
            modal.style.display = 'block';
            const content = document.getElementById('reliability-content');
            setTimeout(() => content.focus(), 10);
            content.innerHTML = 'Loading analysis...';

            fetch('/jobs/analyze/reliability?limit=10')
                .then(res => res.json())
                .then(data => {
                    if (data.total_jobs === 0) {
                        content.innerHTML = '<div style="text-align: center; padding: 2em; color: #555;"><p>No completed jobs found for reliability analysis.</p><div style="margin-top: 15px;"><button type="button" onclick="this.closest(\'.modal\').style.display=\'none\'" style="background-color: #6c757d; color: white; padding: 6px 12px; border: none; border-radius: 4px; cursor: pointer;">Go Back (Esc)</button></div></div>';
                        return;
                    }

                    let html = '<h3>Overall Stats</h3>';
                    html += '<ul>' +
                        '<li>Total Evaluated Jobs: ' + data.total_jobs + '</li>' +
                        '<li>Successful Jobs: <span style="color: #198754">' + data.successful_jobs + ' (' + (data.successful_jobs/data.total_jobs*100).toFixed(2) + '%)</span></li>' +
                        '<li>Flaky Jobs: <span style="color: #b45309">' + data.flaky_jobs + ' (' + data.flakiness_rate.toFixed(2) + '%)</span></li>' +
                        '<li>Failed Jobs: <span style="color: #d32f2f">' + data.failed_jobs + ' (' + data.failure_rate.toFixed(2) + '%)</span></li>' +
                        '<li>Overall Success Rate (incl. Flaky): <strong>' + data.success_rate.toFixed(2) + '%</strong></li>' +
                        '<li>Total Retries Performed: ' + data.total_retries + '</li>' +
                    '</ul>';

                    html += '<h3>Top Flaky Jobs (Succeeded eventually with retries)</h3>';
                    if (data.top_flaky_jobs && data.top_flaky_jobs.length > 0) {
                        html += '<table style="width: 100%; border-collapse: collapse; margin-top: 10px;">';
                        html += '<tr><th style="border: 1px solid #ccc; padding: 8px;">Summary</th><th style="border: 1px solid #ccc; padding: 8px;">Occurrences</th><th style="border: 1px solid #ccc; padding: 8px;">Total Retries</th><th style="border: 1px solid #ccc; padding: 8px;">Avg Retries</th></tr>';
                        data.top_flaky_jobs.forEach(stat => {
                            html += '<tr>' +
                                '<td style="border: 1px solid #ccc; padding: 8px;">' + escapeHTML(stat.summary) + '</td>' +
                                '<td style="border: 1px solid #ccc; padding: 8px;">' + stat.occurrences + '</td>' +
                                '<td style="border: 1px solid #ccc; padding: 8px;">' + stat.total_retries + '</td>' +
                                '<td style="border: 1px solid #ccc; padding: 8px;">' + stat.avg_retries.toFixed(2) + '</td>' +
                            '</tr>';
                        });
                        html += '</table>';
                    } else {
                        html += '<div style="text-align: center; padding: 2em; color: #555;"><p>No flaky jobs found.</p><div style="margin-top: 15px;"><button type="button" onclick="this.closest(\'.modal\').style.display=\'none\'" style="background-color: #6c757d; color: white; padding: 6px 12px; border: none; border-radius: 4px; cursor: pointer;">Close (Esc)</button></div></div>';
                    }

                    html += '<h3 style="margin-top: 20px;">Top Failing Jobs (Failed completely)</h3>';
                    if (data.top_failing_jobs && data.top_failing_jobs.length > 0) {
                        html += '<table style="width: 100%; border-collapse: collapse; margin-top: 10px;">';
                        html += '<tr><th style="border: 1px solid #ccc; padding: 8px;">Summary</th><th style="border: 1px solid #ccc; padding: 8px;">Occurrences</th></tr>';
                        data.top_failing_jobs.forEach(stat => {
                            html += '<tr>' +
                                '<td style="border: 1px solid #ccc; padding: 8px;">' + escapeHTML(stat.summary) + '</td>' +
                                '<td style="border: 1px solid #ccc; padding: 8px;">' + stat.occurrences + '</td>' +
                            '</tr>';
                        });
                        html += '</table>';
                    } else {
                        html += '<div style="text-align: center; padding: 2em; color: #555;"><p>No failing jobs found.</p><div style="margin-top: 15px;"><button type="button" onclick="this.closest(\'.modal\').style.display=\'none\'" style="background-color: #6c757d; color: white; padding: 6px 12px; border: none; border-radius: 4px; cursor: pointer;">Close (Esc)</button></div></div>';
                    }

                    content.innerHTML = html;
                })
                .catch(err => {
                    content.innerHTML = '<div class="error">Failed to fetch analysis: ' + err.message + '</div>';
                });
        }

        function closeReliabilityModal() {
            document.getElementById('reliabilityModal').style.display = 'none';
        }

        function openSearchLogsModal() {
            document.getElementById('searchLogsModal').style.display = 'block';
            setTimeout(() => document.getElementById('search-logs-query').focus(), 10);
        }

        function closeSearchLogsModal() {
            document.getElementById('searchLogsModal').style.display = 'none';
        }

        async function performSearchLogs() {
            const query = document.getElementById('search-logs-query').value.trim();
            const tag = document.getElementById('search-logs-tag').value.trim();
            const status = document.getElementById('search-logs-status').value;
            const contextLines = document.getElementById('search-logs-context').value;
            const resultsDiv = document.getElementById('search-logs-results');
            const btn = document.getElementById('btn-search-logs');

            resultsDiv.style.display = 'block';
            resultsDiv.innerHTML = 'Searching...';

            btn.disabled = true;
            btn.innerHTML = '<span class="spinner" aria-hidden="true"></span> Searching...';

            try {
                let url = '/jobs/search/logs?q=' + encodeURIComponent(query);
                if (tag) url += '&tag=' + encodeURIComponent(tag);
                if (status) url += '&status=' + encodeURIComponent(status);
                if (contextLines && parseInt(contextLines) > 0) url += '&context=' + encodeURIComponent(parseInt(contextLines));

                const res = await fetch(url);

                if (!res.ok) {
                    const errorText = await res.text();
                    resultsDiv.innerHTML = '<span style="color:#d32f2f">Search failed: ' + errorText + '</span>';
                    return;
                }

                const results = await res.json();

                if (!results || results.length === 0) {
                    resultsDiv.innerHTML = '<div style="text-align: center; padding: 2em; color: #bbb;"><p>No matching logs found.</p><div style="margin-top: 15px;"><button type="button" onclick="this.closest(\'.modal\').style.display=\'none\'" style="background-color: #6c757d; color: white; padding: 6px 12px; border: none; border-radius: 4px; cursor: pointer;">Close (Esc)</button></div></div>';
                    return;
                }

                let html = '';
                results.forEach(job => {
                    html += '<div style="margin-bottom: 20px;">';
                    html += '<div style="color: #61dafb; font-weight: bold; font-size: 1.1em;">Job: ' + escapeHTML(job.job_id) + ' <span style="color: #aaa; font-size: 0.9em; font-weight: normal;">(' + escapeHTML(job.status) + ')</span></div>';
                    html += '<div style="color: #ccc; margin-bottom: 5px;">Summary: ' + escapeHTML(job.summary) + '</div>';
                    html += '<div style="background: #111; padding: 10px; border-radius: 3px; border-left: 3px solid #61dafb;">';

                    job.matches.forEach(match => {
                        if (match.context_before) {
                            match.context_before.forEach(ctx => {
                                html += '<div style="display: flex; margin-bottom: 4px;">';
                                html += '<span style="color: #888; margin-right: 15px; user-select: none;">' + ctx.line_number + ':</span>';
                                html += '<span style="color: #aaa; white-space: pre-wrap; word-break: break-all;">' + escapeHTML(ctx.text.trim()) + '</span>';
                                html += '</div>';
                            });
                        }
                        html += '<div style="display: flex; margin-bottom: 4px; background: rgba(230, 219, 116, 0.1);">';
                        html += '<span style="color: #888; margin-right: 15px; user-select: none;">' + match.line_number + ':</span>';
                        html += '<span style="color: #e6db74; white-space: pre-wrap; word-break: break-all; font-weight: bold;">' + escapeHTML(match.text.trim()) + '</span>';
                        html += '</div>';
                        if (match.context_after) {
                            match.context_after.forEach(ctx => {
                                html += '<div style="display: flex; margin-bottom: 4px;">';
                                html += '<span style="color: #888; margin-right: 15px; user-select: none;">' + ctx.line_number + ':</span>';
                                html += '<span style="color: #aaa; white-space: pre-wrap; word-break: break-all;">' + escapeHTML(ctx.text.trim()) + '</span>';
                                html += '</div>';
                            });
                        }
                        if (match.context_before?.length || match.context_after?.length) {
                            html += '<hr style="border: 0; border-bottom: 1px solid #444; margin: 10px 0;">';
                        }
                    });

                    html += '</div></div>';
                });

                resultsDiv.innerHTML = html;

            } catch (err) {
                console.error(err);
                resultsDiv.innerHTML = '<span style="color:#d32f2f">Error performing search: ' + err.message + '</span>';
            } finally {
                btn.disabled = false;
                btn.innerText = 'Search';
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

        // Close modals on Escape key or outside click
        window.addEventListener('keydown', function(event) {
            if (event.key === 'Escape') {
                const modals = document.querySelectorAll('.modal');
                modals.forEach(modal => {
                    if (modal.style.display === 'block') {
                        modal.style.display = 'none';
                        if (modal.id === 'logsModal' && currentLogController) {
                            currentLogController.abort();
                            currentLogController = null;
                        }
                    }
                });
            }
        });

        window.addEventListener('click', function(event) {
            if (event.target.classList.contains('modal')) {
                event.target.style.display = 'none';
                if (event.target.id === 'logsModal' && currentLogController) {
                    currentLogController.abort();
                    currentLogController = null;
                }
            }
        });

        window.addEventListener('keydown', function(event) {
            if (event.target.tagName === 'INPUT' || event.target.tagName === 'TEXTAREA' || event.target.tagName === 'SELECT') return;

            const modals = document.querySelectorAll('.modal');
            for (let i = 0; i < modals.length; i++) {
                if (modals[i].style.display === 'block') return;
            }

            if (event.key === '/') {
                event.preventDefault();
                document.getElementById('job-search').focus();
            } else if (event.key === 'r') {
                document.getElementById('refresh-jobs').click();
            } else if (event.key === 's') {
                event.preventDefault();
                document.getElementById('submitModal').style.display = 'block';
                setTimeout(() => document.getElementById('job-summary').focus(), 10);
            }
        });
    </script>
    <div id="a11y-announcer" class="sr-only" aria-live="polite"></div>
</body>
</html>
`
