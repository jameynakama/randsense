import { page } from 'vitest/browser';
import { afterEach, describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { grammar, sentence } from '#lib/testing/fixtures.js';
import Diagram from './Diagram.svelte';

describe('Diagram', () => {
	afterEach(() => page.viewport(1280, 800));

	it('lists the tree under its grammar labels', async () => {
		render(Diagram, { tree: sentence.tree, grammar });

		const list = page.getByRole('list', { name: 'Sentence diagram' });
		await expect.element(list).toBeVisible();
		await expect.element(list.getByText('noun phrase').first()).toBeInTheDocument();
		await expect.element(list.getByText('transitive verb', { exact: true })).toBeInTheDocument();
		await expect.element(list.getByText('devoured')).toBeVisible();
	});

	it('marks the selected word and its branch', async () => {
		render(Diagram, { tree: sentence.tree, grammar, selectedPath: [0, 0, 1] });

		const current = document.querySelector('[aria-current="true"]');
		expect(current?.textContent).toBe('goose');
		expect(document.querySelectorAll('.label.on')).toHaveLength(4);
	});

	it('offers Zoom only when the tree is wider than the screen', async () => {
		render(Diagram, { tree: sentence.tree, grammar });
		await expect.element(page.getByRole('button', { name: 'Zoom' })).not.toBeInTheDocument();

		await page.viewport(320, 640);
		const zoom = page.getByRole('button', { name: 'Zoom' });
		await expect.element(zoom).toHaveAttribute('aria-pressed', 'false');
		await zoom.click();
		await expect.element(zoom).toHaveAttribute('aria-pressed', 'true');
	});
});
