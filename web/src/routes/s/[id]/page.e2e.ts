import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';

async function newSentence(page: Page): Promise<{ id: string; text: string; star_count: number }> {
	const res = await page.request.get('/api/v1/sentences/random');
	expect(res.ok()).toBe(true);
	return res.json();
}

async function expectNoAxeViolations(page: Page) {
	const { violations } = await new AxeBuilder({ page }).analyze();
	expect(violations).toEqual([]);
}

test('shows a saved sentence with link preview tags', async ({ page }) => {
	const s = await newSentence(page);
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
	const s = await newSentence(page);
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
	const s = await newSentence(page);
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
	const s = await newSentence(page);
	await page.setViewportSize({ width: 320, height: 640 });
	await page.goto(`/s/${s.id}`);
	await page.getByRole('group', { name: s.text }).getByRole('button').first().click();

	const overflow = await page.evaluate(
		() => document.documentElement.scrollWidth - window.innerWidth
	);
	expect(overflow).toBeLessThanOrEqual(0);
});

test('keeps keyboard focus out from under the open word card', async ({ page }) => {
	const s = await newSentence(page);
	await page.goto(`/s/${s.id}`);
	await page.getByRole('group', { name: s.text }).getByRole('button').first().click();
	const card = await page.getByRole('region').boundingBox();

	for (let i = 0; i < 4; i++) {
		await page.keyboard.press('Tab');
		const hidden = await page.evaluate((c) => {
			const el = document.activeElement!;
			if (el.closest('section')) return false;
			const r = el.getBoundingClientRect();
			return r.bottom > c.y && r.top < c.y + c.height && r.right > c.x && r.left < c.x + c.width;
		}, card!);
		expect(hidden).toBe(false);
	}
});
