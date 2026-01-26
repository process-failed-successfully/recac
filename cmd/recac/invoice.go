package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	invoiceSince    string
	invoiceAuthor   string
	invoiceRate     float64
	invoiceClient   string
	invoiceAddress  string
	invoiceNumber   string
	invoiceDue      string
	invoiceTax      float64
	invoiceCurrency string
	invoiceOutput   string
)

var invoiceCmd = &cobra.Command{
	Use:   "invoice",
	Short: "Generate an invoice based on git history",
	Long: `Generates an HTML invoice for work done based on git commit history.
Reuses the session calculation logic from 'timesheet' command.
Can output to stdout or a file.`,
	Example: `  recac invoice --client "Acme Corp" --rate 150 --since 30d > invoice.html
  recac invoice --client "Startup Inc" --address "123 Tech Lane" --tax 20 --currency "EUR"`,
	RunE: runInvoice,
}

func init() {
	rootCmd.AddCommand(invoiceCmd)
	invoiceCmd.Flags().StringVar(&invoiceSince, "since", "30d", "Time window to analyze")
	invoiceCmd.Flags().StringVar(&invoiceAuthor, "author", "", "Filter by author (default: current user)")
	invoiceCmd.Flags().Float64Var(&invoiceRate, "rate", 100, "Hourly rate")
	invoiceCmd.Flags().StringVar(&invoiceClient, "client", "Client Name", "Client name")
	invoiceCmd.Flags().StringVar(&invoiceAddress, "address", "", "Client address (use \\n for newlines)")
	invoiceCmd.Flags().StringVar(&invoiceNumber, "number", "", "Invoice number (default: auto-generated based on date)")
	invoiceCmd.Flags().StringVar(&invoiceDue, "due", "14d", "Due date offset (e.g. 14d) or specific date")
	invoiceCmd.Flags().Float64Var(&invoiceTax, "tax", 0, "Tax percentage (e.g. 20 for 20%)")
	invoiceCmd.Flags().StringVar(&invoiceCurrency, "currency", "USD", "Currency symbol or code")
	invoiceCmd.Flags().StringVarP(&invoiceOutput, "output", "o", "", "Output file path (default: stdout)")
}

func runInvoice(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// 1. Resolve Defaults
	if invoiceAuthor == "" {
		author, err := getGitConfig(cwd, "user.name")
		if err != nil {
			return fmt.Errorf("could not detect git user.name: %w", err)
		}
		invoiceAuthor = author
	}

	if invoiceNumber == "" {
		invoiceNumber = fmt.Sprintf("INV-%s", time.Now().Format("20060102"))
	}

	// Parse Due Date
	var dueDate time.Time
	if strings.HasSuffix(invoiceDue, "d") {
		daysStr := strings.TrimSuffix(invoiceDue, "d")
		if days, err := strconv.Atoi(daysStr); err == nil {
			dueDate = time.Now().AddDate(0, 0, days)
		}
	}

	if dueDate.IsZero() {
		if duration, err := time.ParseDuration(invoiceDue); err == nil {
			dueDate = time.Now().Add(duration)
		} else {
			// Try parsing date
			if t, err := time.Parse("2006-01-02", invoiceDue); err == nil {
				dueDate = t
			} else {
				dueDate = time.Now().AddDate(0, 0, 14) // Default 14 days
			}
		}
	}

	// 2. Fetch Data (Reuse timesheet logic)
	// Default threshold/padding from timesheet defaults if not exposed?
	// The vars `timesheetThreshold` are in `timesheet.go`.
	// We can't easily access the *flag values* of another command unless we parse them.
	// But we can use the default string values.
	threshold, _ := time.ParseDuration("60m")
	padding, _ := time.ParseDuration("30m")

	commits, err := getGitCommits(cwd, invoiceSince, invoiceAuthor)
	if err != nil {
		return err
	}

	sessions := calculateSessions(commits, threshold, padding)
	report := aggregateTimesheet(sessions, invoiceRate)

	// 3. Calculate Totals
	subtotal := report.TotalCost
	taxAmount := subtotal * (invoiceTax / 100)
	total := subtotal + taxAmount

	// 4. Generate HTML
	html := generateInvoiceHTML(report, subtotal, taxAmount, total, dueDate)

	// 5. Output
	if invoiceOutput != "" {
		if err := os.WriteFile(invoiceOutput, []byte(html), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Invoice generated: %s\n", invoiceOutput)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), html)
	}

	return nil
}

func generateInvoiceHTML(report TimesheetReport, subtotal, tax, total float64, dueDate time.Time) string {
	dateStr := time.Now().Format("Jan 02, 2006")
	dueStr := dueDate.Format("Jan 02, 2006")
	address := strings.ReplaceAll(invoiceAddress, "\\n", "<br>")

	// Prepare rows
	var rows strings.Builder
	// Group by day for invoice clarity? Or just one line item?
	// Let's do one line item per day to look professional.
	// Sort dates
	var dates []string
	for d := range report.DailyStats {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	for _, d := range dates {
		hours := report.DailyStats[d]
		amount := hours * invoiceRate
		rows.WriteString(fmt.Sprintf(`
        <tr class="item">
            <td>Development Services - %s</td>
            <td>%.2f</td>
            <td>%.2f</td>
        </tr>
        `, d, hours, amount))
	}

	// If too many rows, maybe summarize?
	if len(dates) > 30 {
		rows.Reset()
		rows.WriteString(fmt.Sprintf(`
        <tr class="item">
            <td>Development Services (Total for period)</td>
            <td>%.2f</td>
            <td>%.2f</td>
        </tr>
        `, report.TotalHours, subtotal))
	}

	css := `
    body { font-family: 'Helvetica Neue', Helvetica, Arial, sans-serif; padding: 20px; color: #555; }
    .invoice-box { max-width: 800px; margin: auto; padding: 30px; border: 1px solid #eee; box-shadow: 0 0 10px rgba(0, 0, 0, .15); }
    .invoice-box table { width: 100%; line-height: inherit; text-align: left; border-collapse: collapse; }
    .invoice-box table td { padding: 5px; vertical-align: top; }
    .invoice-box table tr td:nth-child(2) { text-align: right; }
    .invoice-box table tr td:nth-child(3) { text-align: right; }
    .invoice-box table tr.top table td { padding-bottom: 20px; }
    .invoice-box table tr.top table td.title { font-size: 45px; line-height: 45px; color: #333; }
    .invoice-box table tr.information table td { padding-bottom: 40px; }
    .invoice-box table tr.heading td { background: #eee; border-bottom: 1px solid #ddd; font-weight: bold; }
    .invoice-box table tr.details td { padding-bottom: 20px; }
    .invoice-box table tr.item td { border-bottom: 1px solid #eee; }
    .invoice-box table tr.item.last td { border-bottom: none; }
    .invoice-box table tr.total td:nth-child(3) { border-top: 2px solid #eee; font-weight: bold; }
    `

	template := fmt.Sprintf(`
<!doctype html>
<html>
<head>
    <meta charset="utf-8">
    <title>Invoice %s</title>
    <style>%s</style>
</head>
<body>
    <div class="invoice-box">
        <table cellpadding="0" cellspacing="0">
            <tr class="top">
                <td colspan="3">
                    <table>
                        <tr>
                            <td class="title">INVOICE</td>
                            <td>
                                Invoice #: %s<br>
                                Created: %s<br>
                                Due: %s
                            </td>
                        </tr>
                    </table>
                </td>
            </tr>
            <tr class="information">
                <td colspan="3">
                    <table>
                        <tr>
                            <td>
                                %s<br>
                                %s
                            </td>
                            <td>
                                %s<br>
                                %s
                            </td>
                        </tr>
                    </table>
                </td>
            </tr>
            <tr class="heading">
                <td>Item</td>
                <td>Hours</td>
                <td>Price (%s)</td>
            </tr>
            %s
            <tr class="total">
                <td></td>
                <td>Subtotal:</td>
                <td>%s %.2f</td>
            </tr>
            <tr class="total">
                <td></td>
                <td>Tax (%.0f%%):</td>
                <td>%s %.2f</td>
            </tr>
            <tr class="total">
                <td></td>
                <td>Total:</td>
                <td>%s %.2f</td>
            </tr>
        </table>
    </div>
</body>
</html>
`,
		invoiceNumber, css,
		invoiceNumber, dateStr, dueStr,
		invoiceAuthor, "Freelance Developer", // Sender (Default to author)
		invoiceClient, address,
		invoiceCurrency,
		rows.String(),
		invoiceCurrency, subtotal,
		invoiceTax, invoiceCurrency, tax,
		invoiceCurrency, total,
	)

	return template
}
