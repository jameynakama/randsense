import { createRawSnippet } from 'svelte';
import { page, userEvent } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Tabs from './Tabs.svelte';

const tabs = [
	{ id: 'one', label: 'One' },
	{ id: 'two', label: 'Two' },
	{ id: 'three', label: 'Three' }
];
const panel = createRawSnippet((id: () => string) => ({
	render: () => `<p>Panel ${id()}</p>`
}));
const tab = (name: string) => page.getByRole('tab', { name });

describe('Tabs', () => {
	it('starts on the first tab and shows only its panel', async () => {
		render(Tabs, { label: 'Views', tabs, panel });

		await expect.element(page.getByRole('tablist', { name: 'Views' })).toBeVisible();
		await expect.element(tab('One')).toHaveAttribute('aria-selected', 'true');
		await expect.element(tab('Two')).toHaveAttribute('aria-selected', 'false');
		await expect
			.element(page.getByRole('tabpanel', { name: 'One' }))
			.toHaveTextContent('Panel one');
		await expect.element(page.getByText('Panel two')).not.toBeVisible();
	});

	it('keeps only the selected tab in the tab order', async () => {
		render(Tabs, { label: 'Views', tabs, panel });

		await expect.element(tab('One')).toHaveAttribute('tabindex', '0');
		await expect.element(tab('Two')).toHaveAttribute('tabindex', '-1');
	});

	it('selects a clicked tab', async () => {
		render(Tabs, { label: 'Views', tabs, panel });

		await tab('Two').click();

		await expect.element(tab('Two')).toHaveAttribute('aria-selected', 'true');
		await expect
			.element(page.getByRole('tabpanel', { name: 'Two' }))
			.toHaveTextContent('Panel two');
		await expect.element(page.getByText('Panel one')).not.toBeVisible();
	});

	it('moves with the arrow keys and wraps at the ends', async () => {
		render(Tabs, { label: 'Views', tabs, panel });
		(tab('One').element() as HTMLElement).focus();

		await userEvent.keyboard('{ArrowRight}');
		await expect.element(tab('Two')).toHaveFocus();
		await expect.element(tab('Two')).toHaveAttribute('aria-selected', 'true');

		await userEvent.keyboard('{ArrowLeft}{ArrowLeft}');
		await expect.element(tab('Three')).toHaveFocus();
		await expect.element(tab('Three')).toHaveAttribute('aria-selected', 'true');

		await userEvent.keyboard('{ArrowRight}');
		await expect.element(tab('One')).toHaveFocus();
	});

	it('jumps to the first and last tabs with Home and End', async () => {
		render(Tabs, { label: 'Views', tabs, panel });
		(tab('One').element() as HTMLElement).focus();

		await userEvent.keyboard('{End}');
		await expect.element(tab('Three')).toHaveFocus();

		await userEvent.keyboard('{Home}');
		await expect.element(tab('One')).toHaveFocus();
		await expect.element(tab('One')).toHaveAttribute('aria-selected', 'true');
	});
});
