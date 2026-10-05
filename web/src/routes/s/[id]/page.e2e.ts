import { expect, test } from '@playwright/test';
import {
	expectNoAxeViolations,
	expectNoSidewaysScroll,
	newSentence
} from '../../../lib/testing/e2e';

test('shows a saved sentence with link preview tags', async ({ page }) => {
	const s = await newSentence(page.request);
	await page.goto(`/s/${s.id}`);

	await expect(page.getByRole('group', { name: s.text })).toBeVisible();
	await expect(page.locator('meta[property="og:title"]')).toHaveAttribute('content', s.text);
	await expect(page).toHaveTitle(`${s.text} · RandSense`);
	await expectNoAxeViolations(page);
});

test('an unknown sentence is a 404 page', async ({ page }) => {
	const res = await page.goto('/s/zzzzzzzz');

	expect(res?.status()).toBe(404);
	await expect(page.getByRole('heading', { name: 'No sentence here' })).toBeVisible();
	await expectNoAxeViolations(page);
});

test('opens a word card by keyboard and returns focus on Escape', async ({ page }) => {
	const s = await newSentence(page.request);
	await page.goto(`/s/${s.id}`);
	const first = page.getByRole('group', { name: s.text }).getByRole('button').first();

	await first.focus();
	await page.keyboard.press('Enter');
	await expect(page.getByRole('region')).toBeVisible();
	await expect(page.getByRole('region').getByRole('heading')).toBeFocused();
	await expectNoAxeViolations(page);

	await page.keyboard.press('Escape');
	await expect(page.getByRole('region')).toHaveCount(0);
	await expect(first).toBeFocused();
});

test('stars and unstars, remembering the star across reloads', async ({ page }) => {
	const s = await newSentence(page.request);
	await page.goto(`/s/${s.id}`);

	await page.getByRole('button', { name: `Star, ${s.star_count} stars` }).click();
	const starred = page.getByRole('button', { name: /^Star, 1 star$/ });
	await expect(starred).toHaveAttribute('aria-pressed', 'true');

	await page.reload();
	await expect(starred).toHaveAttribute('aria-pressed', 'true');
	await starred.click();
	await expect(page.getByRole('button', { name: 'Star, 0 stars' })).toHaveAttribute(
		'aria-pressed',
		'false'
	);
});

test('reflows at 320px without sideways scrolling', async ({ page }) => {
	const s = await newSentence(page.request);
	await page.setViewportSize({ width: 320, height: 640 });
	await page.goto(`/s/${s.id}`);
	await page.getByRole('group', { name: s.text }).getByRole('button').first().click();

	await expectNoSidewaysScroll(page);
});

test('keeps keyboard focus out from under the open word card', async ({ page }) => {
	const s = await newSentence(page.request);
	await page.goto(`/s/${s.id}`);
	await page.getByRole('group', { name: s.text }).getByRole('button').first().click();
	const card = page.getByRole('region');
	const focused = page.locator(':focus');

	// Tab through what follows the card, until focus leaves the page.
	for (let i = 0; i < 4; i++) {
		await page.keyboard.press('Tab');
		if ((await focused.count()) === 0) break;
		if ((await card.locator(':focus').count()) > 0) continue;
		const f = (await focused.boundingBox())!;
		const c = (await card.boundingBox())!;
		const overlaps =
			f.y + f.height > c.y && f.y < c.y + c.height && f.x + f.width > c.x && f.x < c.x + c.width;
		expect(overlaps).toBe(false);
	}
});

test('flags a word by keyboard alone', async ({ page }) => {
	const s = await newSentence(page.request);
	await page.goto(`/s/${s.id}`);
	const opener = page.getByRole('button', { name: 'Something’s wrong' });

	await opener.focus();
	await page.keyboard.press('Enter');
	await expect(page.getByRole('heading', { name: 'Report a problem' })).toBeFocused();
	await expectNoAxeViolations(page);
	await page.keyboard.press('Tab');
	await expect(page.getByLabel('About', { exact: true })).toBeFocused();
	// Typing a word's first letter picks it. ArrowDown would, except in
	// Chrome on macOS, where it opens the picker instead.
	await page.keyboard.press(s.text[0]);
	await expect(page.getByLabel('About', { exact: true })).toHaveValue('0');
	await page.keyboard.press('Tab');
	await page.keyboard.type('This word should not be here.');
	await page.keyboard.press('Tab');
	await page.keyboard.press('Enter');

	await expect(page.getByRole('status').filter({ hasText: 'Thanks' })).toBeVisible();
	await page.keyboard.press('Escape');
	await expect(opener).toBeFocused();
});

test('draws the diagram with no accessibility violations', async ({ page }) => {
	const s = await newSentence(page.request);
	await page.goto(`/s/${s.id}`);

	await page.getByRole('button', { name: 'Show diagram' }).click();

	await expect(page.getByRole('list', { name: 'Sentence diagram' })).toBeVisible();
	await expectNoAxeViolations(page);
});

test('lets the keyboard reach a zoomed diagram', async ({ page }) => {
	await page.setViewportSize({ width: 320, height: 640 });
	const s = await newSentence(page.request);
	await page.goto(`/s/${s.id}`);
	await page.getByRole('button', { name: 'Show diagram' }).click();
	const zoom = page.getByRole('button', { name: 'Zoom', exact: true });
	test.skip(!(await zoom.isVisible()), 'this sentence fits at 320px');

	await zoom.click();

	await page.keyboard.press('Tab');
	await expect(page.getByRole('region', { name: 'Sentence diagram, full size' })).toBeFocused();
	await expectNoAxeViolations(page);
});
