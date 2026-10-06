<script lang="ts">
	import { CONTENT_POS, definitions, wiktionaryUrl } from '#lib/definitions.js';
	import { featureLabels, frame, leaves, pos, role } from '#lib/tree.js';
	import type { TreeNode } from '#lib/types.js';

	let { tree, index, onclose }: { tree: TreeNode; index: number; onclose: () => void } = $props();

	const leaf = $derived(leaves(tree)[index]);
	const labels = $derived(featureLabels(leaf.node.features));
	const wordRole = $derived(role(tree, leaf.path));
	const headingId = $props.id();
	const defsId = `${headingId}-defs`;
	let heading: HTMLHeadingElement | undefined = $state();
	// SHOWN is how many senses show before "Show all".
	const SHOWN = 3;
	const lemma = $derived(leaf.node.lemma ?? leaf.node.word ?? '');
	let defs: string[] = $state([]);
	let showAll = $state(false);

	// Each newly opened word moves focus to its heading.
	$effect(() => {
		void index;
		heading?.focus();
	});

	// Each newly opened word fetches its own definitions. An answer for a word
	// no longer open is dropped, and a failed lookup shows none.
	$effect(() => {
		const p = pos(leaf.node).toLowerCase();
		const l = lemma;
		defs = [];
		showAll = false;
		if (!CONTENT_POS.includes(p)) return;
		let current = true;
		definitions(fetch, p, l).then(
			(d) => {
				if (current) defs = d;
			},
			() => {}
		);
		return () => {
			current = false;
		};
	});
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<section class="sheet" aria-labelledby={headingId}>
	<h2 id={headingId} tabindex="-1" bind:this={heading}>{leaf.node.display ?? leaf.node.word}</h2>
	<dl>
		<dt>Lemma</dt>
		<dd>{leaf.node.lemma}</dd>
		<dt>Part of speech</dt>
		<dd>{pos(leaf.node).toLowerCase()}</dd>
		{#if frame(leaf.node)}
			<dt>Frame</dt>
			<dd>{frame(leaf.node)}</dd>
		{/if}
		{#if wordRole}
			<dt>Role</dt>
			<dd>{wordRole}</dd>
		{/if}
	</dl>
	{#if labels.length}
		<ul class="features" aria-label="Features">
			{#each labels as label (label)}
				<li>{label}</li>
			{/each}
		</ul>
	{/if}
	{#if defs.length}
		<h3 id={defsId}>Definitions</h3>
		<ol aria-labelledby={defsId}>
			{#each showAll ? defs : defs.slice(0, SHOWN) as d, i (i)}
				<li>{d}</li>
			{/each}
		</ol>
		{#if defs.length > SHOWN}
			<button
				type="button"
				class="button plain"
				aria-expanded={showAll}
				onclick={() => (showAll = !showAll)}
				>{showAll ? 'Show fewer' : `Show all ${defs.length}`}</button
			>
		{/if}
		<p class="credit">
			Definitions from <a href="https://en-word.net/">Open English WordNet</a> (CC BY 4.0)
		</p>
	{/if}
	<p>
		<a href={wiktionaryUrl(lemma)} target="_blank" rel="noopener">Open in Wiktionary</a>
	</p>
	<button type="button" class="button" onclick={onclose}>Close</button>
</section>

<style>
	h2 {
		margin-block-start: 0;
		color: var(--word-selected);
		font-size: var(--text-xl);
	}

	h3 {
		margin-block-end: 0.25rem;
	}

	ol {
		margin-block-start: 0;
		padding-inline-start: 1.5rem;
	}

	.credit {
		color: var(--secondary);
		font-size: var(--text-sm);
	}

	dl {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr);
		gap: 0.25rem 1rem;
	}

	dt {
		color: var(--secondary);
	}

	dd {
		margin: 0;
	}

	.features {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		padding: 0;
		list-style: none;
	}

	.features li {
		font-size: var(--text-sm);
		background: var(--bg);
		border-radius: 8px;
		padding: 0.25rem 0.75rem;
	}

	@media (max-width: 640px) {
		.features li {
			background: var(--fill);
		}
	}
</style>
