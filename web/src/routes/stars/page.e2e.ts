import { randomUUID } from 'node:crypto';
import { expect, test } from '@playwright/test';
import { FEED_SIZE } from '../../lib/feed';
import { expectNoAxeViolations, newSentence } from '../../lib/testing/e2e';
import type { Sentence } from '../../lib/types';

test('lists what this browser starred', async ({ page }) => {
	const s = await newSentence(page.request);
	await page.goto(`/s/${s.id}`);
	await page.getByRole('button', { name: /^Star, / }).click();
	await expect(page.getByRole('button', { name: /^Star, / })).toHaveAttribute(
		'aria-pressed',
		'true'
	);

	await page.getByRole('navigation').getByRole('link', { name: 'Your stars' }).click();

	await expect(page.getByRole('heading', { level: 1, name: 'Your stars' })).toBeVisible();
	await expect(page.getByRole('link', { name: s.text })).toHaveAttribute('href', `/s/${s.id}`);
	await expectNoAxeViolations(page);
});

test('says so when nothing is starred', async ({ page }) => {
	await page.goto('/stars');

	await expect(
		page.getByText('You haven’t starred any sentences in this browser yet.')
	).toBeVisible();
	await expectNoAxeViolations(page);
});

test('shows more than one page of stars', async ({ page }) => {
	const voter = randomUUID();
	await page.addInitScript((v) => localStorage.setItem('randsense:voter', v), voter);
	// Star sentences that already exist: generating 31 would push the home
	// page tests' sentences out of the feed while they run. Skip the ones
	// still in the feed, since other tests count the stars on theirs.
	const res = await page.request.get(`/api/v1/sentences?limit=31&offset=${FEED_SIZE}`);
	const ids: string[] = (await res.json()).map((s: Sentence) => s.id);
	while (ids.length < 31) ids.push((await newSentence(page.request)).id);
	for (const id of ids) {
		const res = await page.request.post(`/api/v1/sentences/${id}/stars`, {
			headers: { 'X-Voter': voter }
		});
		expect(res.ok()).toBe(true);
	}

	await page.goto('/stars');
	const items = page.getByRole('main').getByRole('listitem');
	await expect(items).toHaveCount(30);
	await page.getByRole('button', { name: 'Show more' }).click();

	await expect(items).toHaveCount(31);
	await expect(page.getByRole('button', { name: 'Show more' })).toHaveCount(0);
});

test('shows each sentence once when stars change between pages', async ({ page }) => {
	const voter = randomUUID();
	await page.addInitScript((v) => localStorage.setItem('randsense:voter', v), voter);
	const res = await page.request.get(`/api/v1/sentences?limit=32&offset=${FEED_SIZE}`);
	const ids: string[] = (await res.json()).map((s: Sentence) => s.id);
	while (ids.length < 32) ids.push((await newSentence(page.request)).id);
	const star = async (id: string) => {
		const res = await page.request.post(`/api/v1/sentences/${id}/stars`, {
			headers: { 'X-Voter': voter }
		});
		expect(res.ok()).toBe(true);
	};
	for (const id of ids.slice(0, 31)) await star(id);

	await page.goto('/stars');
	const links = page.getByRole('main').getByRole('listitem').getByRole('link');
	await expect(links).toHaveCount(30);
	// A star from another tab shifts every later page by one.
	await star(ids[31]);
	await page.getByRole('button', { name: 'Show more' }).click();

	// The second page repeats one sentence from the first and adds one.
	await expect(links).toHaveCount(31);
	const hrefs = await links.evaluateAll((as) => as.map((a) => a.getAttribute('href')));
	expect(new Set(hrefs).size).toBe(hrefs.length);
});
