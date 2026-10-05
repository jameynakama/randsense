import { expect, test } from '@playwright/test';
import { expectNoAxeViolations } from '../../../lib/testing/e2e';

test('logs in by keyboard and out again', async ({ page }) => {
	await page.goto('/admin/login');
	await expect(page.getByRole('heading', { level: 1, name: 'Admin login' })).toBeVisible();
	await expectNoAxeViolations(page);

	await page.getByLabel('Password').fill('randsense');
	await page.keyboard.press('Enter');

	await expect(page).toHaveURL('/admin');
	await expect(page.getByRole('heading', { level: 1, name: 'Flags' })).toBeVisible();

	await page.getByRole('button', { name: 'Log out' }).click();
	await expect(page).toHaveURL('/admin/login');
	await page.goto('/admin');
	await expect(page).toHaveURL('/admin/login');
});

test('says when the password is wrong', async ({ page }) => {
	await page.goto('/admin/login');
	const field = page.getByLabel('Password');

	await field.fill('not the password');
	await page.getByRole('button', { name: 'Log in' }).click();

	await expect(field).toHaveAccessibleDescription('That password isn’t right.');
	await expect(field).toHaveAttribute('aria-invalid', 'true');
	await expect(page).toHaveURL('/admin/login');
	await expectNoAxeViolations(page);
});

test('asks for a password before sending one', async ({ page }) => {
	await page.goto('/admin/login');

	await page.getByRole('button', { name: 'Log in' }).click();

	await expect(page.getByLabel('Password')).toHaveAccessibleDescription('Enter the password.');
	await expect(page.getByLabel('Password')).toBeFocused();
});
