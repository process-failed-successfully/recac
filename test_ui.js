const { chromium } = require('playwright');
const fs = require('fs');
const path = require('path');

(async () => {
    const browser = await chromium.launch();
    const page = await browser.newPage();
    const htmlContent = fs.readFileSync(path.join(__dirname, 'internal/orchestrator/webui.go'), 'utf8');
    const htmlMatches = htmlContent.match(/const DashboardHTML = `([\s\S]*?)`/);

    if (htmlMatches && htmlMatches.length > 1) {
        let html = htmlMatches[1];
        fs.writeFileSync('temp_ui.html', html);
        await page.goto('file://' + path.join(__dirname, 'temp_ui.html'));
        await page.waitForTimeout(2000);

        // Wait, the review said I didn't add the HTML element.
        // But `grep` shows it exists on line 2034! Let's check `test_ui.js` output.
    }
    await browser.close();
})();
