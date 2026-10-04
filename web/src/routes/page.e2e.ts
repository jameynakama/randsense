import { expect, test, type Page } from '@playwright/test';
import { expectNoAxeViolations, expectNoSidewaysScroll, newSentence } from '../lib/testing/e2e';

// Other tests generate sentences at the same time, so these look for a
// sentence anywhere in the feed rather than on top.
const inFeed = (page: Page, text: string) =>
	page
		.getByRole('region', { name: 'Latest sentences' })
		.getByRole('link', { name: text, exact: true });

// ownSentence waits for the sentence the page generated on arrival.
async function ownSentence(page: Page): Promise<string> {
	const group = page.getByRole('group');
	await expect(group).toBeVisible();
	return (await group.getAttribute('aria-label'))!;
}

test('greets a visitor with a fresh sentence that the feed shows too', async ({ page }) => {
	await page.goto('/');

	await expect(inFeed(page, await ownSentence(page))).toBeVisible();
	await expect(page.getByRole('banner').getByRole('link', { name: 'RandSense' })).toBeVisible();
	await expectNoAxeViolations(page);
});

test('generates another sentence and announces it', async ({ page }) => {
	await page.goto('/');
	const first = await ownSentence(page);

	await page.getByRole('button', { name: 'Generate' }).click();

	const group = page.getByRole('group');
	await expect(group).not.toHaveAttribute('aria-label', first);
	const second = (await group.getAttribute('aria-label'))!;
	await expect(page.locator('[aria-live="polite"]', { hasText: second })).toHaveCount(1);
});

test('shows sentences generated elsewhere as they arrive', async ({ page }) => {
	await page.goto('/');
	// The page's own sentence in the feed shows the stream is connected.
	await expect(inFeed(page, await ownSentence(page))).toBeVisible();

	const elsewhere = await newSentence(page.request);

	await expect(inFeed(page, elsewhere.text)).toBeVisible();
});

test('holds the feed still while paused', async ({ page }) => {
	await page.goto('/');
	await expect(inFeed(page, await ownSentence(page))).toBeVisible();

	await page.getByRole('button', { name: 'Pause feed' }).click();
	const elsewhere = await newSentence(page.request);
	await expect(page.getByText(/new sentences? waiting/)).toBeVisible();
	await expect(inFeed(page, elsewhere.text)).toHaveCount(0);

	await page.getByRole('button', { name: 'Pause feed' }).click();
	await expect(inFeed(page, elsewhere.text)).toBeVisible();
});

test('reflows the home page at 320px without sideways scrolling', async ({ page }) => {
	await page.setViewportSize({ width: 320, height: 640 });
	await page.goto('/');
	await ownSentence(page);

	await expectNoSidewaysScroll(page);
});
