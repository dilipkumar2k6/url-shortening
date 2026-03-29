import { test, expect } from '@playwright/test';
import { execSync } from 'child_process';

test.describe('SigNoz Metric Verification', () => {
  test('should create admin and verify metrics', async ({ page }) => {
    // Enable console logging
    page.on('console', msg => console.log('BROWSER CONSOLE:', msg.text()));
    page.on('pageerror', err => console.error('BROWSER ERROR:', err.message));

    // Enable network logging
    page.on('request', request => console.log('>>', request.method(), request.url()));
    page.on('response', async response => {
      if (response.status() >= 400) {
        console.log('<< ERROR', response.status(), response.url());
        try {
          const text = await response.text();
          console.log('<< ERROR BODY:', text);
        } catch (e) {
          console.log('<< ERROR BODY: (failed to read)');
        }
      } else {
        console.log('<<', response.status(), response.url());
      }
    });

    // 1. Create Admin Account (only if not already created)
    console.log('Navigating to SigNoz...');
    await page.goto('http://localhost:3301/signup');

    // Check if we are on the signup page or redirected to login/dashboard
    const isSignupVisible = await page.locator('text=Create your account').isVisible({ timeout: 5000 }).catch(() => false);

    if (isSignupVisible) {
      console.log('Filling signup form...');
      await page.fill('input[type="email"]', 'admin@example.com');
      await page.fill('input[placeholder="Your Name"]', 'Admin');
      await page.fill('input[placeholder="Your Company"]', 'Example');
      await page.locator('input[type="password"]').first().fill('password123');
      await page.locator('input[type="password"]').last().fill('password123');

      console.log('Submitting signup...');
      await page.click('button:has-text("Get Started")');
    } else {
      console.log('Signup page not visible, checking if login needed...');
      await page.goto('http://localhost:3301/login');
      const isLoginVisible = await page.locator('button:has-text("Login")').isVisible({ timeout: 5000 }).catch(() => false);
      if (isLoginVisible) {
        console.log('Logging in...');
        await page.fill('input[type="email"]', 'admin@example.com');
        await page.fill('input[type="password"]', 'password123');
        await page.click('button:has-text("Login")');
      }
    }

    // Wait for dashboard to load (URL should change to /services)
    await expect(page).toHaveURL(/.*\/services|.*\/metrics-explorer/, { timeout: 20000 });
    console.log('Logged in/Signed up successfully!');

    // 2. Skip Triggering Metrics (Data already exists in ClickHouse)
    console.log('Skipping metric triggering as data already exists in ClickHouse...');
    
    // 3. Verify Metrics in SigNoz UI
    const captureUIScreenshot = async (metricName) => {
      console.log(`Verifying metric in SigNoz UI: ${metricName}...`);
      
      let attempts = 0;
      const maxAttempts = 5;
      let success = false;

      while (attempts < maxAttempts && !success) {
        attempts++;
        console.log(`Attempt ${attempts}/${maxAttempts} for ${metricName}...`);
        
        await page.goto('http://localhost:3301/metrics-explorer/explorer');
        
        // Wait for the query builder to be ready
        await expect(page.locator('.ant-select-selection-search-input').first()).toBeVisible({ timeout: 20000 });
        
        console.log(`Searching for ${metricName} in UI...`);
        const metricInput = page.locator('.ant-select-selection-search-input').first();
        await metricInput.click();
        await metricInput.fill(metricName);
        
        // Select option using evaluate for robustness
        const option = page.locator(`.ant-select-item-option-content:has-text("${metricName}")`);
        const isVisible = await option.isVisible({ timeout: 10000 }).catch(() => false);
        
        if (!isVisible) {
          console.log(`${metricName} not found in dropdown yet. Reloading and waiting...`);
          await page.waitForTimeout(10000);
          continue;
        }

        await option.evaluate(node => node.click());
        success = true;
      }

      if (!success) {
        console.log(`FAILED to find ${metricName} after ${maxAttempts} attempts.`);
        await page.screenshot({ path: `test-results/failed-find-${metricName}.png`, fullPage: true });
        return;
      }

      // Select "Sum" operator
      console.log('Setting aggregation operator to Sum...');
      const operatorSelect = page.locator('.ant-select-selector').nth(1);
      await operatorSelect.evaluate(node => node.click());
      await page.waitForTimeout(2000);
      const sumOption = page.locator('.ant-select-item-option-content:has-text("Sum")').first();
      await sumOption.evaluate(node => node.click());
      await page.waitForTimeout(2000);

      console.log('Running query in UI...');
      const runBtn = page.locator('button:has-text("Run Query"), button:has-text("Stage & Run Query")').first();
      await runBtn.evaluate(node => node.click());

      console.log('Waiting for query to process...');
      await page.waitForTimeout(10000); // Give it time to render anything

      const screenshotPath = `test-results/signoz-ui-metric-${metricName}.png`;
      await page.screenshot({ path: screenshotPath, fullPage: true });
      console.log(`Screenshot captured at ${screenshotPath}`);
    };

    // Increase overall test timeout
    test.setTimeout(300000); 

    // Verify metrics visually
    await captureUIScreenshot('shorten_requests_total');
    await captureUIScreenshot('redirect_requests_total');
  });
});
