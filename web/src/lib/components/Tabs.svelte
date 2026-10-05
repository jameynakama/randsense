<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		label,
		tabs,
		panel
	}: { label: string; tabs: { id: string; label: string }[]; panel: Snippet<[string]> } = $props();

	const uid = $props.id();
	// svelte-ignore state_referenced_locally
	let selected = $state(tabs[0].id);
	const buttons: HTMLButtonElement[] = $state([]);

	// Arrow keys, Home and End move between tabs, selecting as they go.
	function onkeydown(e: KeyboardEvent, i: number) {
		const last = tabs.length - 1;
		const moves: Record<string, number> = {
			ArrowRight: i === last ? 0 : i + 1,
			ArrowLeft: i === 0 ? last : i - 1,
			Home: 0,
			End: last
		};
		const next = moves[e.key];
		if (next === undefined) return;
		e.preventDefault();
		selected = tabs[next].id;
		buttons[next].focus();
	}
</script>

<div class="tablist" role="tablist" aria-label={label}>
	{#each tabs as t, i (t.id)}
		<button
			type="button"
			role="tab"
			id="{uid}-{t.id}-tab"
			class="button plain"
			aria-selected={t.id === selected}
			aria-controls="{uid}-{t.id}-panel"
			tabindex={t.id === selected ? 0 : -1}
			bind:this={buttons[i]}
			onclick={() => (selected = t.id)}
			onkeydown={(e) => onkeydown(e, i)}>{t.label}</button
		>
	{/each}
</div>
{#each tabs as t (t.id)}
	<!-- Hidden rather than removed, so each panel keeps what it loaded. -->
	<div
		role="tabpanel"
		id="{uid}-{t.id}-panel"
		aria-labelledby="{uid}-{t.id}-tab"
		tabindex="0"
		hidden={t.id !== selected}
	>
		{@render panel(t.id)}
	</div>
{/each}

<style>
	.tablist {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		margin-block: 1rem;
		border-block-end: 1px solid var(--separator);
	}

	/* The underline marks the selected tab with more than 3:1 contrast. */
	[role='tab'] {
		border-radius: 0;
		border-block-end: 3px solid transparent;
	}

	[role='tab'][aria-selected='true'] {
		color: var(--text);
		border-block-end-color: var(--action);
	}
</style>
