import { page, userEvent } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { grammar } from '#lib/testing/fixtures.js';
import ExpansionSheet from './ExpansionSheet.svelte';

const handlers = () => ({ onchoose: vi.fn(), onclear: vi.fn(), onclose: vi.fn() });

describe('ExpansionSheet', () => {
	it('lists a hole’s rules by their labels and picks one', async () => {
		const h = handlers();
		render(ExpansionSheet, { symbol: 'NP', grammar, filled: false, ...h });

		const sheet = page.getByRole('region', { name: 'Choose noun phrase' });
		await expect.element(sheet.getByText('Names a thing.')).toBeInTheDocument();
		await sheet.getByRole('button', { name: 'determiner + noun', exact: true }).click();
		expect(h.onchoose).toHaveBeenCalledWith(['Determiner', 'Noun']);
		await expect.element(sheet.getByRole('button', { name: 'Clear' })).not.toBeInTheDocument();
	});

	it('shows a verb frame’s example', async () => {
		render(ExpansionSheet, { symbol: 'VP', grammar, filled: false, ...handlers() });

		await expect
			.element(page.getByRole('button', { name: 'transitive verb + noun phrase', exact: true }))
			.toBeInTheDocument();
		await expect.element(page.getByText('“devoured the goose”')).toBeInTheDocument();
	});

	it('offers Clear on a filled phrase', async () => {
		const h = handlers();
		render(ExpansionSheet, { symbol: 'VP', grammar, filled: true, ...h });

		await page
			.getByRole('region', { name: 'Change verb phrase' })
			.getByRole('button', { name: 'Clear' })
			.click();
		expect(h.onclear).toHaveBeenCalledOnce();
	});

	it('takes focus and closes on Escape', async () => {
		const h = handlers();
		render(ExpansionSheet, { symbol: 'NP', grammar, filled: false, ...h });

		await expect.element(page.getByRole('heading', { name: 'Choose noun phrase' })).toHaveFocus();
		await userEvent.keyboard('{Escape}');
		expect(h.onclose).toHaveBeenCalledOnce();
	});
});
