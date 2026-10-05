<script lang="ts">
	import { short } from '#lib/diagram.js';
	import type { Grammar } from '#lib/types.js';

	let {
		symbol,
		grammar,
		filled,
		onchoose,
		onclear,
		onclose
	}: {
		symbol: string;
		grammar: Grammar;
		filled: boolean;
		onchoose: (rule: string[]) => void;
		onclear: () => void;
		onclose: () => void;
	} = $props();

	const phrase = $derived(grammar.phrases[symbol]);
	const headingId = $props.id();
	let heading: HTMLHeadingElement | undefined = $state();

	// Each newly opened phrase moves focus to its heading.
	$effect(() => {
		void symbol;
		heading?.focus();
	});

	function label(s: string): string {
		return (grammar.phrases[s] ?? grammar.slots[s])?.label ?? s;
	}

	function example(rule: string[]): string | undefined {
		return rule.map((s) => grammar.slots[s]?.example).find(Boolean);
	}
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<section class="sheet" aria-labelledby={headingId}>
	<h2 id={headingId} tabindex="-1" bind:this={heading}>
		{filled ? 'Change' : 'Choose'}
		{phrase.label}
	</h2>
	<p class="description">{phrase.description}</p>
	<ul class="rules">
		{#each phrase.rules as rule, i (i)}
			{@const e = example(rule)}
			<li>
				<button type="button" class="rule" onclick={() => onchoose(rule)}
					><span>{rule.map(label).join(' + ')}</span><span class="symbols" aria-hidden="true"
						>{rule.map((s) => short({ symbol: s })).join(' ')}</span
					></button
				>
				{#if e}<p class="example">“{e}”</p>{/if}
			</li>
		{/each}
	</ul>
	<div class="actions">
		{#if filled}
			<button type="button" class="button plain" onclick={onclear}>Clear</button>
		{/if}
		<button type="button" class="button" onclick={onclose}>Close</button>
	</div>
</section>

<style>
	h2 {
		margin-block-start: 0;
	}

	.description {
		color: var(--secondary);
	}

	.rules {
		display: grid;
		gap: 0.5rem;
		padding: 0;
		list-style: none;
	}

	.rule {
		display: flex;
		flex-wrap: wrap;
		justify-content: space-between;
		align-items: center;
		gap: 0.25rem 1rem;
		inline-size: 100%;
		min-block-size: 44px;
		padding: 0.5rem 1rem;
		font: inherit;
		font-weight: 600;
		text-align: start;
		color: var(--action);
		background: var(--bg);
		border: none;
		border-radius: 12px;
		cursor: pointer;
	}

	.symbols {
		font-size: var(--text-sm);
		color: var(--secondary);
	}

	.example {
		margin: 0.25rem 1rem 0;
		font-size: var(--text-sm);
		color: var(--secondary);
	}

	.actions {
		display: flex;
		gap: 0.5rem;
		justify-content: flex-end;
	}

	@media (max-width: 640px) {
		.rule {
			background: var(--fill);
		}
	}
</style>
