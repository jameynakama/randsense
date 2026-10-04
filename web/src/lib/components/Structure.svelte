<script lang="ts">
	import type { TreeNode } from '#lib/types.js';
	import Branch from './Branch.svelte';

	let {
		tree,
		selectedPath = null,
		open = $bindable(false)
	}: { tree: TreeNode; selectedPath?: number[] | null; open?: boolean } = $props();

	const outlineId = $props.id();
</script>

<div class="structure">
	<button
		type="button"
		class="pill"
		aria-expanded={open}
		aria-controls={outlineId}
		onclick={() => (open = !open)}>{open ? 'Hide structure' : 'Show structure'}</button
	>
	<ul id={outlineId} class="outline" hidden={!open} aria-label="Sentence structure">
		<Branch node={tree} path={[]} {selectedPath} />
	</ul>
</div>

<style>
	.outline {
		text-align: start;
		padding: 0;
		margin-block: 1rem;
		overflow-x: auto;
	}
</style>
