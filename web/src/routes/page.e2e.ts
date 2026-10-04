import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';

test('the home page has its heading and no axe violations', async ({ page }) => {
	await page.goto('/');

	await expect(page.getByRole('heading', { level: 1, name: 'RandSense' })).toBeVisible();
	await expect(page.getByRole('banner').getByRole('link', { name: 'RandSense' })).toBeVisible();
	await expect(page.getByRole('main')).toBeVisible();
	const { violations } = await new AxeBuilder({ page }).analyze();
	expect(violations).toEqual([]);
});
