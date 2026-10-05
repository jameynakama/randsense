import { page } from 'vitest/browser';
import { afterEach, describe, expect, it, vi } from 'vitest';
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

	describe('editing', () => {
		// The second clause's verb phrase is a hole.
		const tree = structuredClone(sentence.tree);
		tree.children![3].children![1] = { symbol: 'VP' };
		tree.children![0].children![0].children![0].locked = true;
		const editing = () => ({ onphrase: vi.fn(), onword: vi.fn(), problemPath: null });

		it('makes holes and phrases buttons that open their rules', async () => {
			const e = editing();
			render(Diagram, { tree, grammar, editing: e });

			await page.getByRole('button', { name: 'Choose verb phrase' }).click();
			expect(e.onphrase).toHaveBeenCalledWith([3, 1]);
			await page.getByRole('button', { name: 'Change verb phrase' }).click();
			expect(e.onphrase).toHaveBeenCalledWith([0, 1]);
		});

		it('makes lockable words lock toggles, and leaves fixed words alone', async () => {
			const e = editing();
			render(Diagram, { tree, grammar, editing: e });

			await expect
				.element(page.getByRole('button', { name: 'the', exact: true }))
				.toHaveAttribute('aria-pressed', 'true');
			const goose = page.getByRole('button', { name: 'goose', exact: true });
			await expect.element(goose).toHaveAttribute('aria-pressed', 'false');
			await goose.click();
			expect(e.onword).toHaveBeenCalledWith([0, 0, 1]);
			await expect
				.element(page.getByRole('button', { name: ',', exact: true }))
				.not.toBeInTheDocument();
		});

		it('marks the problem slot, a hole included', async () => {
			render(Diagram, { tree, grammar, editing: { ...editing(), problemPath: [3, 1] } });

			await expect
				.element(page.getByRole('button', { name: 'Choose verb phrase' }))
				.toHaveClass(/problem/);
		});

		it('never shrinks, so its targets stay 44px', async () => {
			await page.viewport(320, 640);
			render(Diagram, { tree, grammar, editing: editing() });

			await expect.element(page.getByRole('button', { name: 'Zoom' })).not.toBeInTheDocument();
			const box = page
				.getByRole('button', { name: 'goose', exact: true })
				.element()
				.getBoundingClientRect();
			expect(box.height).toBeGreaterThanOrEqual(44);
			expect(box.width).toBeGreaterThanOrEqual(44);
		});
	});
});
