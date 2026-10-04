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

<section class="card" aria-labelledby={headingId}>
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
	<button type="button" class="pill" onclick={onclose}>Close</button>
</section>

<style>
	.card {
		text-align: start;
		border: 2px solid var(--outline);
		border-radius: 1rem;
		padding: 1rem 1.5rem;
		background: var(--bg);
		max-inline-size: 40rem;
		margin-inline: auto;
		overflow-wrap: anywhere;
	}

	h2 {
		margin-block-start: 0;
		color: var(--word);
		font-size: 2rem;
	}

	dl {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr);
		gap: 0.25rem 1rem;
	}

	dt {
		font-weight: bold;
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
		border: 1px solid var(--outline);
		border-radius: 1rem;
		padding: 0.1rem 0.75rem;
	}

	@media (max-width: 640px) {
		/* Room to scroll everything above the sheet, and focus scrolled clear
		   of it, so the sheet never hides a focused control. */
		:global(html:has(.card)) {
			scroll-padding-block-end: 60vh;
		}

		:global(body:has(.card)) {
			padding-block-end: 60vh;
		}

		.card {
			position: fixed;
			inset-inline: 0;
			inset-block-end: 0;
			max-block-size: 60vh;
			overflow-y: auto;
			border-radius: 1rem 1rem 0 0;
			margin: 0;
			box-shadow: 0 -0.25rem 1rem rgb(0 0 0 / 0.2);
		}
	}
</style>
