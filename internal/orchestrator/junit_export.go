package orchestrator

import (
	"encoding/xml"
	"fmt"
	"time"
)

// JUnitReport represents a JUnit XML report.
type JUnitReport struct {
	XMLName    xml.Name       `xml:"testsuites"`
	TestSuites []JUnitTestSuite `xml:"testsuite"`
}

// JUnitTestSuite represents a single test suite in a JUnit XML report.
type JUnitTestSuite struct {
	Name      string          `xml:"name,attr"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	Errors    int             `xml:"errors,attr"`
	Time      string          `xml:"time,attr"`
	TestCases []JUnitTestCase `xml:"testcase"`
}

// JUnitTestCase represents a single test case in a JUnit XML report.
type JUnitTestCase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *JUnitFailure `xml:"failure,omitempty"`
}

// JUnitFailure represents a failure in a test case.
type JUnitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Content string `xml:",chardata"`
}

// ExportJobsToJUnitXML converts a list of jobs to a JUnit XML format.
func ExportJobsToJUnitXML(jobs []JobInfo) (string, error) {
	suite := JUnitTestSuite{
		Name:      "RECAC Jobs",
		Tests:     len(jobs),
		Failures:  0,
		Errors:    0,
		TestCases: []JUnitTestCase{},
	}

	var totalDuration float64

	for _, job := range jobs {
		var dur float64
		if !job.StartTime.IsZero() {
			if !job.EndTime.IsZero() {
				dur = job.EndTime.Sub(job.StartTime).Seconds()
			} else {
				dur = time.Since(job.StartTime).Seconds()
			}
		}
		totalDuration += dur

		tc := JUnitTestCase{
			Name:      job.ID,
			Classname: "orchestrator.jobs",
			Time:      fmt.Sprintf("%.3f", dur),
		}

		if job.Status == "Failed" || job.Status == "Error" || job.Status == "Missing" || job.Status == "Canceled" {
			suite.Failures++
			var failureMsg string
			if job.StatusMessage != nil {
				failureMsg = *job.StatusMessage
			} else {
				failureMsg = job.Summary
			}
			tc.Failure = &JUnitFailure{
				Message: failureMsg,
				Type:    job.Status,
				Content: "Job execution failed or was canceled.",
			}
		}

		suite.TestCases = append(suite.TestCases, tc)
	}

	suite.Time = fmt.Sprintf("%.3f", totalDuration)

	report := JUnitReport{
		TestSuites: []JUnitTestSuite{suite},
	}

	out, err := xml.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}

	return xml.Header + string(out), nil
}
