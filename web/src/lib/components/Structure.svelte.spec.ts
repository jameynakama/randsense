import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { sentence } from '#lib/testing/fixtures.js';
import Structure from './Structure.svelte';

describe('Structure', () => {
	it('opens and closes the outline', async () => {
		render(Structure, { tree: sentence.tree });
		const toggle = page.getByRole('button', { name: 'Show structure' });

		// Hidden, so it's out of the accessibility tree until opened.
		await expect
			.element(page.getByRole('list', { name: 'Sentence structure' }))
			.not.toBeInTheDocument();
		await toggle.click();
		await expect.element(page.getByRole('list', { name: 'Sentence structure' })).toBeVisible();
		await expect
			.element(page.getByRole('button', { name: 'Hide structure' }))
			.toHaveAttribute('aria-expanded', 'true');
		await expect
			.element(page.getByRole('button', { name: 'NP, noun phrase' }).first())
			.toBeVisible();
	});

	it('collapses a branch', async () => {
		render(Structure, { tree: sentence.tree, open: true });
		const vp = page.getByRole('button', { name: 'VP, verb phrase' }).first();

		await vp.click();
		await expect.element(vp).toHaveAttribute('aria-expanded', 'false');
		await expect.element(page.getByText('devoured')).not.toBeVisible();
	});

	it('marks the selected word and its branch', async () => {
		render(Structure, { tree: sentence.tree, open: true, selectedPath: [0, 0, 1] });

		const current = page.getByText('goose').element().closest('[aria-current]');
		expect(current?.getAttribute('aria-current')).toBe('true');
		expect(document.querySelectorAll('li.on-branch')).toHaveLength(4);
	});
});
