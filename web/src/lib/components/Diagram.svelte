<script lang="ts">
	import { onMount } from 'svelte';
	import { lockable } from '#lib/builder.js';
	import { EDIT, key, layout, READ, short, type Editing, type Measure } from '#lib/diagram.js';
	import type { Grammar, TreeNode } from '#lib/types.js';

	let {
		tree,
		grammar,
		selectedPath = null,
		editing = null
	}: {
		tree: TreeNode;
		grammar: Grammar;
		selectedPath?: number[] | null;
		editing?: Editing | null;
	} = $props();

	// Kept in step with .label and .word below, since the layout measures text
	// in these fonts.
	const fonts = {
		label: "600 13px 'Atkinson Hyperlegible Next Variable', sans-serif",
		word: "500 17px 'Atkinson Hyperlegible Next Variable', sans-serif"
	};
	let context: CanvasRenderingContext2D | null = null;
	// Measuring before the web font arrives would use the fallback's widths,
	// so the layout runs again once it has.
	let fontsReady = $state(false);
	onMount(() => {
		document.fonts.ready.then(() => (fontsReady = true));
	});
	// Editable labels and words are buttons: padded, at least 44px wide, and a
	// label has room for a hole's " ?".
	const measure: Measure = (text, kind) => {
		context ??= document.createElement('canvas').getContext('2d')!;
		context.font = fonts[kind];
		const width = context.measureText(text).width;
		return editing ? Math.max(44, width + (kind === 'label' ? 40 : 36)) : width;
	};

	const drawn = $derived.by(() => {
		void fontsReady;
		return layout(tree, measure, editing ? EDIT : READ);
	});
	let available = $state(0);
	let zoomed = $state(false);
	// Until the width is measured, assume the tree fits, so Zoom doesn't
	// flash in and out on the first frame.
	const fits = $derived(!available || drawn.width <= available);
	// An editable tree never shrinks, so its targets keep their size; it
	// scrolls sideways instead.
	const scale = $derived(editing || zoomed || fits ? 1 : available / drawn.width);

	function same(a: number[] | null | undefined, b: number[]): boolean {
		return !!a && a.length === b.length && a.every((v, i) => b[i] === v);
	}

	function onBranch(path: number[]): boolean {
		return (
			selectedPath !== null &&
			path.length <= selectedPath.length &&
			path.every((v, i) => selectedPath[i] === v)
		);
	}

	function name(node: TreeNode): string {
		return (grammar.phrases[node.symbol] ?? grammar.slots[node.symbol])?.label ?? node.symbol;
	}
</script>

{#snippet branch(node: TreeNode, path: number[])}
	{@const placed = drawn.placed.get(key(path))!}
	{@const problem = same(editing?.problemPath, path)}
	<li>
		{#if editing && node.symbol in grammar.phrases}
			{@const hole = !node.children?.length}
			<button
				type="button"
				id="slot-{key(path)}"
				class="label slot"
				class:hole
				class:problem
				style:left="{placed.label.x}px"
				style:top="{placed.label.y}px"
				onclick={() => editing?.onphrase(path)}
				><span aria-hidden="true">{short(node)}{hole ? ' ?' : ''}</span><span
					class="visually-hidden">{hole ? 'Choose' : 'Change'} {name(node)}</span
				></button
			>
		{:else}
			<span
				class="label"
				class:on={onBranch(path)}
				class:problem
				style:left="{placed.label.x}px"
				style:top="{placed.label.y}px"
				><span aria-hidden="true">{short(node)}</span><span class="visually-hidden"
					>{name(node)}</span
				></span
			>
		{/if}
		{#if placed.word}
			{#if editing && lockable(node)}
				<button
					type="button"
					class="word lock"
					aria-pressed={!!node.locked}
					style:left="{placed.word.x}px"
					style:top="{placed.word.y}px"
					onclick={() => editing?.onword(path)}
					>{placed.word.text}{#if node.locked}<svg
							class="lock-icon"
							aria-hidden="true"
							viewBox="0 0 12 14"
							width="12"
							height="14"
							><rect x="1" y="6" width="10" height="8" rx="1.5" fill="currentColor" /><path
								d="M3.5 6V4a2.5 2.5 0 0 1 5 0v2"
								fill="none"
								stroke="currentColor"
								stroke-width="1.5"
							/></svg
						>{/if}</button
				>
			{:else}
				<span
					class="word"
					aria-current={onBranch(path) && path.length === selectedPath?.length ? 'true' : undefined}
					style:left="{placed.word.x}px"
					style:top="{placed.word.y}px">{placed.word.text}</span
				>
			{/if}
		{/if}
		{#if node.children?.length}
			<ul>
				{#each node.children as child, i (i)}
					{@render branch(child, [...path, i])}
				{/each}
			</ul>
		{/if}
	</li>
{/snippet}

<div class="diagram" class:editing={!!editing}>
	<!-- Measures the width alone: the diagram's own height changes with Zoom. -->
	<div bind:clientWidth={available}></div>
	{#if !editing && !fits}
		<button
			type="button"
			class="button plain"
			aria-pressed={zoomed}
			onclick={() => (zoomed = !zoomed)}>Zoom</button
		>
	{/if}
	<!-- Zoomed, the tree scrolls sideways, so the keyboard needs a way in. An
	editable tree's buttons take focus, so its viewport needs no tabindex. -->
	<div
		class="viewport"
		class:zoomed={zoomed || !!editing}
		style:height="{drawn.height * scale}px"
		tabindex={zoomed && !editing ? 0 : undefined}
		role={zoomed && !editing ? 'region' : undefined}
		aria-label={zoomed && !editing ? 'Sentence diagram, full size' : undefined}
	>
		<div class="canvas" style:width="{drawn.width}px" style:height="{drawn.height}px" style:scale>
			<svg aria-hidden="true" width={drawn.width} height={drawn.height}>
				{#each drawn.edges as e, i (i)}
					<line
						x1={e.from[0]}
						y1={e.from[1]}
						x2={e.to[0]}
						y2={e.to[1]}
						class:dotted={e.dotted}
						class:on={onBranch(e.path)}
					/>
				{/each}
			</svg>
			<ul aria-label="Sentence diagram">
				{@render branch(tree, [])}
			</ul>
		</div>
	</div>
</div>

<style>
	.diagram {
		margin-block: 1rem;
	}

	.viewport {
		overflow: hidden;
	}

	.viewport.zoomed {
		overflow-x: auto;
	}

	.canvas {
		position: relative;
		margin-inline: auto;
		transform-origin: 0 0;
	}

	svg,
	.canvas > ul {
		position: absolute;
		inset: 0;
	}

	line {
		stroke: var(--separator);
		stroke-width: 1.5;
	}

	line.dotted {
		stroke-dasharray: 2 3;
	}

	line.on {
		stroke: var(--word-selected);
		stroke-width: 2.5;
	}

	ul {
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.label,
	.word {
		position: absolute;
		translate: -50% 0;
		white-space: nowrap;
	}

	.label {
		font:
			600 13px/20px 'Atkinson Hyperlegible Next Variable',
			sans-serif;
		color: var(--secondary);
	}

	.label.on {
		color: var(--word-selected);
	}

	.word {
		font:
			500 17px/24px 'Atkinson Hyperlegible Next Variable',
			sans-serif;
		color: var(--text);
	}

	.editing .label,
	.editing .word {
		line-height: 44px;
	}

	.slot,
	.lock {
		min-block-size: 44px;
		min-inline-size: 44px;
		border-radius: 8px;
		cursor: pointer;
	}

	.slot {
		padding: 0 12px;
		color: var(--text);
		background: var(--bg);
		border: 1.5px solid var(--separator);
	}

	.slot.hole {
		color: var(--action);
		border-style: dashed;
		border-color: var(--action);
	}

	.lock {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 0 8px;
		background: none;
		border: none;
	}

	.lock[aria-pressed='true'] {
		color: var(--action);
		background: var(--action-tint);
	}

	.label.problem,
	.slot.problem {
		color: var(--error);
		border-color: var(--error);
	}

	.word[aria-current='true'] {
		color: var(--word-selected);
		text-decoration: underline;
		text-underline-offset: 0.15em;
	}
</style>
