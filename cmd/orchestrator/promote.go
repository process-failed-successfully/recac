package main

func promoteBulkJobs(host, match, tag, group string) {
	jobBulkAction(host, "promote", match, tag, group, "Successfully promoted")
}

func promoteJob(host, jobID string) {
	jobAction(host, jobID, "promote", "Successfully promoted")
}
