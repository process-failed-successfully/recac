package main

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	researchLimit int
)

var researchBaseURL = "https://html.duckduckgo.com/html/"

// researchHttpClient allows mocking in tests
var researchHttpClient = &http.Client{
	Timeout: 15 * time.Second,
}

var researchCmd = &cobra.Command{
	Use:   "research [query]",
	Short: "Research a topic using internet search and AI",
	Long: `Performs a web search for the given query, scrapes the top results,
and uses the AI agent to summarize the findings and answer your question.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runResearch,
}

func init() {
	rootCmd.AddCommand(researchCmd)
	researchCmd.Flags().IntVarP(&researchLimit, "limit", "l", 3, "Number of search results to analyze")
}

func runResearch(cmd *cobra.Command, args []string) error {
	query := strings.Join(args, " ")
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// 1. Search
	fmt.Fprintf(cmd.OutOrStdout(), "🔍 Searching the web for: %s\n", query)
	results, err := searchDuckDuckGo(query, researchLimit)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No results found.")
		return nil
	}

	// 2. Fetch Content
	var contentBuilder strings.Builder
	contentBuilder.WriteString(fmt.Sprintf("Research Topic: %s\n\n", query))

	for i, res := range results {
		fmt.Fprintf(cmd.OutOrStdout(), "📄 Reading [%d/%d]: %s\n", i+1, len(results), res.Title)
		text, err := fetchPageContent(res.URL)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to fetch %s: %v\n", res.URL, err)
			continue
		}

		// Truncate text to avoid blowing up context window
		if len(text) > 4000 {
			text = text[:4000] + "... (truncated)"
		}

		contentBuilder.WriteString(fmt.Sprintf("--- SOURCE %d: %s ---\nURL: %s\nCONTENT:\n%s\n\n", i+1, res.Title, res.URL, text))
	}

	// 3. Summarize with Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-research")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are a Technical Researcher.
Answer the user's query based on the following gathered information.
Cite sources where appropriate (e.g., [Source 1]).

Query: %s

Gathered Information:
%s
`, query, contentBuilder.String())

	fmt.Fprintln(cmd.OutOrStdout(), "\n🧠 Synthesizing answer...")
	fmt.Fprintln(cmd.OutOrStdout(), "") // Newline

	_, err = ag.SendStream(ctx, prompt, func(chunk string) {
		fmt.Fprint(cmd.OutOrStdout(), chunk)
	})
	fmt.Fprintln(cmd.OutOrStdout(), "")

	return err
}

type SearchResult struct {
	Title string
	URL   string
}

func searchDuckDuckGo(query string, limit int) ([]SearchResult, error) {
	// Use html.duckduckgo.com to avoid JS requirement
	values := url.Values{}
	values.Set("q", query)

	req, err := http.NewRequest("POST", researchBaseURL, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := researchHttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("search API returned status: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	body := string(bodyBytes)

	// Naive regex parsing for DDG HTML
	// Look for <a class="result__a" href="...">Title</a>
	// The class might vary, but result__a is common in the HTML version.
	// Or look for <a href="http..." class="result__a">

	// Regex: <a[^>]*class="[^"]*result__a[^"]*"[^>]*href="([^"]+)"[^>]*>(.*?)</a>
	re := regexp.MustCompile(`<a[^>]*class="[^"]*result__a[^"]*"[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)

	matches := re.FindAllStringSubmatch(body, limit+5) // Fetch a few more to filter ads/internal

	var results []SearchResult
	for _, m := range matches {
		if len(results) >= limit {
			break
		}

		link := m[1]
		title := m[2]

		// Clean title (strip tags inside title if any)
		title = stripTags(title)
		title = html.UnescapeString(title)

		// Decode URL if needed (DDG sometimes wraps them)
		// Usually raw in HTML version, but check
		if strings.HasPrefix(link, "/html/") {
			continue // Internal link
		}
		// DDG sometimes uses /l/?kh=-1&uddg=...
		if strings.Contains(link, "duckduckgo.com/l/") {
			// Extract uddg param
			u, _ := url.Parse(link)
			if u != nil {
				uddg := u.Query().Get("uddg")
				if uddg != "" {
					link = uddg
				}
			}
		}

		results = append(results, SearchResult{
			Title: title,
			URL:   link,
		})
	}

	return results, nil
}

func fetchPageContent(pageURL string) (string, error) {
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; RecacBot/1.0)")

	resp, err := researchHttpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return cleanPageText(string(bodyBytes)), nil
}

func cleanPageText(htmlContent string) string {
	// 1. Remove scripts and styles
	reScript := regexp.MustCompile(`(?s)<script.*?>.*?</script>`)
	reStyle := regexp.MustCompile(`(?s)<style.*?>.*?</style>`)
	htmlContent = reScript.ReplaceAllString(htmlContent, "")
	htmlContent = reStyle.ReplaceAllString(htmlContent, "")

	// 2. Strip tags
	text := stripTags(htmlContent)

	// 3. Unescape
	text = html.UnescapeString(text)

	// 4. Collapse whitespace
	reSpace := regexp.MustCompile(`\s+`)
	text = reSpace.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

func stripTags(content string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(content, " ")
}
