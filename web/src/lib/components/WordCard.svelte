<script lang="ts">
	import { featureLabels, frame, leaves, pos, role } from '#lib/tree.js';
	import type { TreeNode } from '#lib/types.js';

	let { tree, index, onclose }: { tree: TreeNode; index: number; onclose: () => void } = $props();

	const leaf = $derived(leaves(tree)[index]);
	const labels = $derived(featureLabels(leaf.node.features));
	const wordRole = $derived(role(tree, leaf.path));
	const headingId = $props.id();
	let heading: HTMLHeadingElement | undefined = $state();

	// Each newly opened word moves focus to its heading.
	$effect(() => {
		void index;
		heading?.focus();
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
	<button type="button" class="button" onclick={onclose}>Close</button>
</section>

<style>
	h2 {
		margin-block-start: 0;
		color: var(--word-selected);
		font-size: var(--text-xl);
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
