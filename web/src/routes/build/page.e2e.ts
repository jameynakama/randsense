import { expect, test, type Page } from '@playwright/test';
import { expectNoAxeViolations, expectNoSidewaysScroll } from '../../lib/testing/e2e';
import { leaves } from '../../lib/tree';
import type { TreeNode } from '../../lib/types';

const diagram = (page: Page) => page.getByRole('list', { name: 'Sentence diagram' });

// pick opens a slot's sheet and picks a rule, by keyboard.
async function pick(page: Page, slot: string, rule: string) {
	await diagram(page)
		.getByRole('button', { name: `Choose ${slot}`, exact: true })
		.focus();
	await page.keyboard.press('Enter');
	const sheet = page.getByRole('region', { name: `Choose ${slot}` });
	await expect(sheet.getByRole('heading')).toBeFocused();
	await sheet.getByRole('button', { name: rule, exact: true }).focus();
	await page.keyboard.press('Enter');
	await expect(sheet).toHaveCount(0);
}

// build makes "Det Noun Verb" by keyboard, fills it and returns the
// realize response.
async function build(page: Page): Promise<{ text: string; tree: TreeNode }> {
	await page.goto('/build');
	await pick(page, 'sentence', 'clause');
	await pick(page, 'clause', 'noun phrase + verb phrase');
	await pick(page, 'noun phrase', 'determiner + noun');
	await pick(page, 'verb phrase', 'intransitive verb');
	const filled = page.waitForResponse('**/api/v1/sentences/realize');
	await page.getByRole('button', { name: 'Fill', exact: true }).focus();
	await page.keyboard.press('Enter');
	return (await filled).json();
}

test('builds, fills, locks and rerolls a sentence by keyboard', async ({ page }) => {
	const first = await build(page);
	await expect(page.getByText(first.text, { exact: true })).toBeVisible();

	// Det, Noun and Verb are the word toggles, in order.
	const noun = diagram(page).locator('button[aria-pressed]').nth(1);
	await noun.focus();
	await page.keyboard.press('Enter');
	await expect(noun).toHaveAttribute('aria-pressed', 'true');

	const rerolled = page.waitForResponse('**/api/v1/sentences/realize');
	await page.getByRole('button', { name: 'Reroll', exact: true }).focus();
	await page.keyboard.press('Enter');
	const second: { tree: TreeNode } = await (await rerolled).json();
	const lemma = (r: { tree: TreeNode }) =>
		leaves(r.tree).find((l) => l.node.symbol === 'Noun')!.node.lemma;
	expect(lemma(second)).toBe(lemma(first));
});

test('returns focus to the phrase when the sheet closes on Escape', async ({ page }) => {
	await page.goto('/build');
	const slot = diagram(page).getByRole('button', { name: 'Choose sentence', exact: true });
	await slot.focus();
	await page.keyboard.press('Enter');
	await expect(page.getByRole('region', { name: 'Choose sentence' })).toBeVisible();

	await page.keyboard.press('Escape');
	await expect(page.getByRole('region', { name: 'Choose sentence' })).toHaveCount(0);
	await expect(slot).toBeFocused();
});

test('undoes a fill', async ({ page }) => {
	await build(page);
	await page.getByRole('button', { name: 'Undo', exact: true }).click();

	await expect(page.getByRole('button', { name: 'Fill', exact: true })).toBeVisible();
});

test('passes axe with the sheet open and with a filled diagram', async ({ page }) => {
	await page.goto('/build');
	await diagram(page).getByRole('button', { name: 'Choose sentence', exact: true }).click();
	await expectNoAxeViolations(page);

	await page.keyboard.press('Escape');
	await page.getByRole('button', { name: 'Fill', exact: true }).click();
	await expect(page.getByRole('button', { name: 'Reroll', exact: true })).toBeVisible();
	await expectNoAxeViolations(page);
});

test('reflows at 320px without sideways scrolling', async ({ page }) => {
	await page.setViewportSize({ width: 320, height: 640 });
	await page.goto('/build');
	await page.getByRole('button', { name: 'Fill', exact: true }).click();
	await expect(page.getByRole('button', { name: 'Reroll', exact: true })).toBeVisible();

	await expectNoSidewaysScroll(page);
});

test('the header links to the builder', async ({ page }) => {
	await page.goto('/stars');
	await page.getByRole('navigation').getByRole('link', { name: 'Build', exact: true }).click();

	await expect(page.getByRole('heading', { name: 'Build a sentence', level: 1 })).toBeVisible();
});
