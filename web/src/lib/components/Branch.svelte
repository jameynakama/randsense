<script lang="ts">
	import { symbolName } from '#lib/tree.js';
	import type { TreeNode } from '#lib/types.js';
	import Branch from './Branch.svelte';

	let {
		node,
		path,
		selectedPath
	}: { node: TreeNode; path: number[]; selectedPath: number[] | null } = $props();

	let open = $state(true);
	const kids = $derived(node.children ?? []);
	const onBranch = $derived(selectedPath !== null && path.every((v, i) => selectedPath[i] === v));
	const isSelected = $derived(onBranch && selectedPath!.length === path.length);
</script>

<li class:on-branch={onBranch}>
	{#if kids.length}
		<button
			type="button"
			class="node"
			aria-expanded={open}
			aria-label="{node.symbol}, {symbolName(node)}"
			onclick={() => (open = !open)}>{node.symbol}</button
		>
		<ul hidden={!open}>
			{#each kids as child, i (i)}
				<Branch node={child} path={[...path, i]} {selectedPath} />
			{/each}
		</ul>
	{:else}
		<span class="leaf" aria-current={isSelected ? 'true' : undefined}
			>{node.symbol} <span class="leaf-word">{node.display ?? node.word}</span></span
		>
	{/if}
</li>

<style>
	li {
		list-style: none;
		padding-inline-start: 1rem;
		border-inline-start: 2px solid transparent;
	}

	li.on-branch {
		border-inline-start-color: var(--word-selected);
	}

	.node {
		font: inherit;
		background: none;
		border: none;
		color: var(--text);
		min-block-size: 44px;
		cursor: pointer;
	}

	.node::before {
		content: '▸ ';
	}

	.node[aria-expanded='true']::before {
		content: '▾ ';
	}

	.leaf {
		display: inline-block;
		min-block-size: 44px;
		line-height: 44px;
	}

	.leaf-word {
		color: var(--accent);
	}

	.leaf[aria-current='true'] .leaf-word {
		font-weight: bold;
		text-decoration: underline;
	}

	ul {
		padding: 0;
		margin: 0;
	}

	@media (max-width: 640px) {
		li {
			padding-inline-start: 0.5rem;
		}
	}
</style>
