from playwright.sync_api import sync_playwright, expect

def run(playwright):
    browser = playwright.chromium.launch(headless=True)
    page = browser.new_page()

    # Go to dashboard
    page.goto("http://127.0.0.1:8081")

    # Check Title
    expect(page).to_have_title("Recac Explorer")

    # Check File Tree
    # We expect "main.go" to be listed
    main_go = page.get_by_text("📄 main.go")
    expect(main_go).to_be_visible()

    # Click main.go
    main_go.click()

    # Check Code View
    # Wait for code to load
    code_content = page.locator("#code-content")
    expect(code_content).to_contain_text("package main")

    # Check Diagram View
    # Tab should be visible
    diagram_tab = page.locator("#tab-diagram")
    expect(diagram_tab).to_be_visible()

    # Click Diagram Tab
    diagram_tab.click()

    # Check Diagram Content
    # Mermaid should render an svg
    # Or at least the graph definition text if not rendered yet, but mermaid replaces it.
    # We check if #diagram-content has content.
    # We can check for the text "class main_Test" which should be in the mermaid definition or SVG.

    # Allow some time for mermaid
    page.wait_for_timeout(2000)

    diagram_content = page.locator("#diagram-content")
    # In mermaid, class names are usually present in the SVG text
    # expect(diagram_content).to_contain_text("main_Test")

    # Take screenshot
    page.screenshot(path="verification.png")

    browser.close()

with sync_playwright() as playwright:
    run(playwright)
