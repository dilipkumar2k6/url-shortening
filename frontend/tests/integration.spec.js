import { test, expect } from '@playwright/test';

test.describe('URL Shortener Integration', () => {
  test.beforeEach(async ({ page }) => {
    page.on('console', msg => console.log('BROWSER CONSOLE:', msg.text()));
    page.on('pageerror', err => console.error('BROWSER ERROR:', err.message));
    // Go to the home page
    await page.goto('/');
  });

  test('should shorten a URL and show it in the list', async ({ page }) => {
    const longUrl = `https://example.com/test-${Date.now()}`;

    // Fill the input
    await page.fill('input[placeholder*="https://very-long-url.com/some/path/to/resource"]', longUrl);

    // Click the shorten button
    await page.click('button:has-text("Shorten")');

    // Wait for the short URL to appear (it should contain the dynamic domain)
    const resultBox = page.locator('.font-mono.text-blue-600');
    await expect(resultBox).toBeVisible();
    await expect(resultBox).toContainText('localhost:10001');

    const shortUrl = await resultBox.innerText();

    // Step 2: Click the short URL to generate analytics
    // We use a new page or just navigate to it
    await page.goto(shortUrl);
    // Wait for redirect to happen (it should go to the longUrl)
    await expect(page).toHaveURL(longUrl);

    // Step 3: Check if the long URL is in the analytics table
    // We need to wait for Flink and ClickHouse to process the event
    await page.goto('/');
    // Wait up to 20 seconds for the analytics to appear
    await expect(page.locator(`text=${longUrl}`)).toBeVisible({ timeout: 20000 });
  });

  test('should display analytics dashboard', async ({ page }) => {
    // Check for the dashboard title
    await expect(page.locator('text=Top Performing Links')).toBeVisible();

    // Check if the table has headers
    await expect(page.locator('th:has-text("Short Link")')).toBeVisible();
    await expect(page.locator('th:has-text("Destination")')).toBeVisible();
    await expect(page.locator('th:has-text("Clicks")')).toBeVisible();
  });

  test('should shorten a URL with custom slug when logged in', async ({ page }) => {
    // 1. Open Auth Modal and Login as guest
    await page.click('button:has-text("Sign In")');
    await page.click('button:has-text("Continue as Guest")');
    // Wait for modal to close and user state to update
    await expect(page.locator('text=Guest')).toBeVisible();

    const longUrl = `https://example.com/auth-test-${Date.now()}`;
    const customSlug = `custom-${Date.now()}`;

    // 2. Fill the long URL
    await page.fill('input[placeholder*="https://very-long-url.com/some/path/to/resource"]', longUrl);

    // 3. Fill the custom slug
    await page.fill('input[placeholder="Custom alias (optional)"]', customSlug);

    // 4. Click the shorten button
    await page.click('button:has-text("Shorten")');

    // 5. Verify the result
    const resultBox = page.locator('.font-mono.text-blue-600');
    await expect(resultBox).toBeVisible({ timeout: 10000 });
    await expect(resultBox).toContainText(customSlug);

    // 6. Verify redirect
    const shortUrl = await resultBox.innerText();
    // We need to use the full URL including protocol for navigation
    const fullShortUrl = shortUrl.startsWith('http') ? shortUrl : `http://${shortUrl}`;
    await page.goto(fullShortUrl);
    await expect(page).toHaveURL(longUrl);
  });

  test('should show user links in "My Links" page', async ({ page }) => {
    // 1. Login as guest
    await page.click('button:has-text("Sign In")');
    await page.click('button:has-text("Continue as Guest")');
    await expect(page.locator('text=Guest')).toBeVisible();

    const longUrl = `https://example.com/history-test-${Date.now()}`;

    // 2. Shorten a URL
    await page.fill('input[placeholder*="https://very-long-url.com/some/path/to/resource"]', longUrl);
    await page.click('button:has-text("Shorten")');
    await expect(page.locator('.font-mono.text-blue-600')).toBeVisible();

    // 3. Navigate to "My Links"
    await page.click('nav >> text=My Links');
    await expect(page).toHaveURL('/my-links');

    // 4. Verify the link is in the list
    // Use a more robust locator and wait for it to appear
    await expect(page.locator(`[title="${longUrl}"]`)).toBeVisible({ timeout: 10000 });
  });

  test('should show multiple user links in "My Links" page in correct order', async ({ page }) => {
    // 1. Login as guest
    await page.click('button:has-text("Sign In")');
    await page.click('button:has-text("Continue as Guest")');
    await expect(page.locator('text=Guest')).toBeVisible();

    const longUrl1 = `https://example.com/history-1-${Date.now()}`;
    const longUrl2 = `https://example.com/history-2-${Date.now()}`;

    // 2. Shorten first URL
    await page.fill('input[placeholder*="https://very-long-url.com/some/path/to/resource"]', longUrl1);
    await page.click('button:has-text("Shorten")');
    await expect(page.locator('.font-mono.text-blue-600')).toBeVisible();

    // 3. Shorten second URL
    await page.fill('input[placeholder*="https://very-long-url.com/some/path/to/resource"]', longUrl2);
    await page.click('button:has-text("Shorten")');
    await expect(page.locator('.font-mono.text-blue-600')).toBeVisible();

    // 4. Navigate to "My Links"
    await page.click('nav >> text=My Links');
    await expect(page).toHaveURL('/my-links');

    // 5. Verify both links are in the list and in correct order (recent first)
    const rows = page.locator('table tbody tr');
    // We might have more links from previous tests if the DB is not cleared, 
    // but at least these two should be there in correct relative order.

    // Find the indices of our two links
    const text1 = await rows.locator(`[title="${longUrl1}"]`).isVisible();
    const text2 = await rows.locator(`[title="${longUrl2}"]`).isVisible();

    await expect(page.locator(`[title="${longUrl1}"]`)).toBeVisible();
    await expect(page.locator(`[title="${longUrl2}"]`)).toBeVisible();

    // Verify ordering: longUrl2 should appear BEFORE longUrl1 in the DOM
    const content = await page.content();
    const index1 = content.indexOf(longUrl1);
    const index2 = content.indexOf(longUrl2);
    expect(index2).toBeLessThan(index1);
  });

  test('should edit and delete a link from the "My Links" page', async ({ page }) => {
    // 1. Login as guest
    await page.click('button:has-text("Sign In")');
    await page.click('button:has-text("Continue as Guest")');
    await expect(page.locator('text=Guest')).toBeVisible();

    const initialUrl = `https://example.com/edit-test-${Date.now()}`;
    const updatedUrl = `https://example.com/updated-${Date.now()}`;

    // 2. Shorten a URL
    await page.fill('input[placeholder*="https://very-long-url.com/some/path/to/resource"]', initialUrl);
    await page.click('button:has-text("Shorten")');
    await expect(page.locator('.font-mono.text-blue-600')).toBeVisible();

    // 3. Navigate to "My Links"
    await page.click('nav >> text=My Links');
    await expect(page).toHaveURL('/my-links');

    // 4. Get Short URL and Generate Clicks
    const row = page.locator('tr', { hasText: initialUrl });
    // Find the link that contains the short URL
    const shortUrlLink = row.locator('a[href*="localhost:10001"]');
    const shortUrl = await shortUrlLink.innerText();
    const fullShortUrl = shortUrl.startsWith('http') ? shortUrl : `http://${shortUrl}`;

    // Generate 3 clicks to ensure it appears in top links
    for (let i = 0; i < 3; i++) {
      await page.goto(fullShortUrl);
      await expect(page).toHaveURL(initialUrl);
    }

    // Wait for analytics aggregation and verify in Top Links on Home page
    await page.goto('/');
    await expect(page.locator(`text=${initialUrl}`)).toBeVisible({ timeout: 20000 });

    // Go back to My Links to Edit
    await page.click('nav >> text=My Links');
    await expect(page).toHaveURL('/my-links');

    // 5. Click the Edit button
    const editRow = page.locator('tr', { hasText: initialUrl });
    await editRow.locator('button[title="Edit link"]').click();

    // 6. Update the URL in the modal
    await expect(page.locator('text=Edit Link')).toBeVisible();
    const input = page.locator('input[placeholder="https://example.com/very-long-url"]');
    await input.fill(updatedUrl);
    await page.click('button:has-text("Save Changes")');

    // 7. Verify the update in the list
    await expect(page.locator(`[title="${updatedUrl}"]`)).toBeVisible({ timeout: 10000 });
    await expect(page.locator(`[title="${initialUrl}"]`)).not.toBeVisible();

    // 8. Verify Redirect to New URL
    await page.goto(fullShortUrl);
    await expect(page).toHaveURL(updatedUrl);

    // Go back to My Links to Delete
    await page.goto('/my-links');

    // 9. Delete the link
    // Handle the confirmation dialog
    page.on('dialog', dialog => dialog.accept());
    await page.locator('tr', { hasText: updatedUrl }).locator('button[title="Delete link"]').click();

    // 10. Verify the link is gone from list
    await expect(page.locator(`[title="${updatedUrl}"]`)).not.toBeVisible({ timeout: 10000 });

    // 11. Verify removed from Top Links on Home page
    await page.goto('/');
    await expect(page.locator(`text=${updatedUrl}`)).not.toBeVisible({ timeout: 10000 });
  });
});
