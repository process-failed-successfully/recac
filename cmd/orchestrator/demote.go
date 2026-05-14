package main

func demoteBulkJobs(host, match, tag, group string) {
	jobBulkAction(host, "demote", match, tag, group, "Successfully demoted")
}

func demoteJob(host, jobID string) {
	jobAction(host, jobID, "demote", "Successfully demoted")
}
