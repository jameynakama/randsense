<script lang="ts">
	import { onMount } from 'svelte';
	import { key, layout, short, type Measure } from '#lib/diagram.js';
	import type { Grammar, TreeNode } from '#lib/types.js';

	let {
		tree,
		grammar,
		selectedPath = null
	}: { tree: TreeNode; grammar: Grammar; selectedPath?: number[] | null } = $props();

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
	const measure: Measure = (text, kind) => {
		context ??= document.createElement('canvas').getContext('2d')!;
		context.font = fonts[kind];
		return context.measureText(text).width;
	};

	const drawn = $derived.by(() => {
		void fontsReady;
		return layout(tree, measure);
	});
	let available = $state(0);
	let zoomed = $state(false);
	// Until the width is measured, assume the tree fits, so Zoom doesn't
	// flash in and out on the first frame.
	const fits = $derived(!available || drawn.width <= available);
	const scale = $derived(zoomed || fits ? 1 : available / drawn.width);

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
	<li>
		<span
			class="label"
			class:on={onBranch(path)}
			style:left="{placed.label.x}px"
			style:top="{placed.label.y}px"
			><span aria-hidden="true">{short(node)}</span><span class="visually-hidden">{name(node)}</span
			></span
		>
		{#if placed.word}
			<span
				class="word"
				aria-current={onBranch(path) && path.length === selectedPath?.length ? 'true' : undefined}
				style:left="{placed.word.x}px"
				style:top="{placed.word.y}px">{placed.word.text}</span
			>
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

<div class="diagram">
	<!-- Measures the width alone: the diagram's own height changes with Zoom. -->
	<div bind:clientWidth={available}></div>
	{#if !fits}
		<button
			type="button"
			class="button plain"
			aria-pressed={zoomed}
			onclick={() => (zoomed = !zoomed)}>Zoom</button
		>
	{/if}
	<div class="viewport" class:zoomed style:height="{drawn.height * scale}px">
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

	.word[aria-current='true'] {
		color: var(--word-selected);
		text-decoration: underline;
		text-underline-offset: 0.15em;
	}
</style>
