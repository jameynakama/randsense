import { randomUUID } from 'node:crypto';
import { expect, test, type Page } from '@playwright/test';
import {
	expectNoAxeViolations,
	expectNoSidewaysScroll,
	flagSentence,
	logIn,
	newSentence
} from '../../lib/testing/e2e';
import { leaves } from '../../lib/tree';

// flagMany adds more than a page of flags, so "Show more" appears.
async function flagMany(page: Page) {
	const s = await newSentence(page.request);
	for (let i = 0; i < 31; i++) await flagSentence(page.request, s.id, `e2e page filler ${i}`);
}

test('sends a visitor without a session to log in', async ({ page }) => {
	await page.goto('/admin');

	await expect(page).toHaveURL('/admin/login');
});

test('lists the newest flags with the flagged word', async ({ page }) => {
	await logIn(page);
	const s = await newSentence(page.request);
	// The last leaf is always a word, never a comma.
	const index = leaves(s.tree).length - 1;
	const { node } = leaves(s.tree)[index];
	const comment = `e2e ${randomUUID()}`;
	await flagSentence(page.request, s.id, comment, index);

	await page.goto('/admin');

	await expect(page.getByRole('heading', { level: 1, name: 'Flags' })).toBeVisible();
	const item = page.getByRole('listitem').filter({ hasText: comment });
	await expect(item.getByRole('link', { name: s.text, exact: true })).toHaveAttribute(
		'href',
		`/s/${s.id}`
	);
	await expect(item).toContainText(`About “${node.display ?? node.word}”`);
	await expectNoAxeViolations(page);
});

test('switches to the most-flagged words by keyboard', async ({ page }) => {
	await logIn(page);
	const s = await newSentence(page.request);
	await flagSentence(page.request, s.id, `e2e ${randomUUID()}`, 0);
	await page.goto('/admin');

	await page.getByRole('tab', { name: 'Newest flags' }).focus();
	await page.keyboard.press('ArrowRight');

	const tab = page.getByRole('tab', { name: 'Most-flagged words' });
	await expect(tab).toBeFocused();
	await expect(tab).toHaveAttribute('aria-selected', 'true');
	const panel = page.getByRole('tabpanel', { name: 'Most-flagged words' });
	await expect(panel.getByRole('columnheader', { name: 'Flags' })).toBeVisible();
	await expect(panel.getByRole('row').nth(1)).toBeVisible();
	await expectNoAxeViolations(page);
});

test('shows more flags', async ({ page }) => {
	await logIn(page);
	await flagMany(page);
	await page.goto('/admin');
	const items = page.getByRole('tabpanel').getByRole('listitem');
	await expect(items).toHaveCount(30);

	await page.getByRole('button', { name: 'Show more' }).click();

	await expect.poll(() => items.count()).toBeGreaterThan(30);
});

test('sends the admin to log in when the session ends', async ({ page, context }) => {
	await logIn(page);
	await flagMany(page);
	await page.goto('/admin');
	await expect(page.getByRole('button', { name: 'Show more' })).toBeVisible();

	await context.clearCookies();
	await page.getByRole('button', { name: 'Show more' }).click();

	await expect(page).toHaveURL('/admin/login');
});

test('fits a long unbroken comment on a narrow screen', async ({ page }) => {
	await page.setViewportSize({ width: 320, height: 640 });
	await logIn(page);
	const s = await newSentence(page.request);
	await flagSentence(page.request, s.id, 'x'.repeat(1000));

	await page.goto('/admin');

	await expect(page.getByRole('listitem').filter({ hasText: 'xxxxxxxxxx' }).first()).toBeVisible();
	await expectNoSidewaysScroll(page);
});
