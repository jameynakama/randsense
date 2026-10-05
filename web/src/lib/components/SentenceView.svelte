<script lang="ts">
	import { tick } from 'svelte';
	import { leaves } from '#lib/tree.js';
	import type { Grammar, Sentence as SentenceData } from '#lib/types.js';
	import Diagram from './Diagram.svelte';
	import FlagForm from './FlagForm.svelte';
	import Sentence, { wordId } from './Sentence.svelte';
	import StarButton from './StarButton.svelte';
	import WordCard from './WordCard.svelte';

	let {
		sentence,
		grammar,
		count = $bindable()
	}: { sentence: SentenceData; grammar: Grammar; count: number } = $props();

	// Both reset when another sentence takes this one's place.
	let selected = $derived.by<number | null>(() => {
		void sentence.id;
		return null;
	});
	let flagging = $derived.by(() => {
		void sentence.id;
		return false;
	});
	const selectedPath = $derived(selected === null ? null : leaves(sentence.tree)[selected].path);
	let flagButton: HTMLButtonElement | undefined = $state();
	// Stays open from one sentence to the next.
	let showDiagram = $state(false);

	async function closeCard() {
		const opener = selected;
		selected = null;
		await tick();
		if (opener !== null) document.getElementById(wordId(sentence.id, opener))?.focus();
	}

	async function closeFlagForm() {
		// The form's target was only a target, not a word to show.
		selected = null;
		flagging = false;
		await tick();
		flagButton?.focus();
	}
</script>

<Sentence {sentence} bind:selected />
{#if sentence.origin === 'built'}<p class="badge">Homemade</p>{/if}

<div class="actions">
	<StarButton id={sentence.id} bind:count />
	<button
		type="button"
		class="button plain"
		aria-expanded={flagging}
		bind:this={flagButton}
		onclick={() => (flagging = !flagging)}>Something’s wrong</button
	>
</div>

<!-- While the flag form is open, pressing a word picks its target instead of opening its card. -->
{#if flagging}
	<FlagForm {sentence} bind:target={selected} onclose={closeFlagForm} />
{:else if selected !== null}
	<WordCard tree={sentence.tree} index={selected} onclose={closeCard} />
{/if}

<button
	type="button"
	class="button plain"
	aria-expanded={showDiagram}
	onclick={() => (showDiagram = !showDiagram)}
	>{showDiagram ? 'Hide diagram' : 'Show diagram'}</button
>
<a class="button plain" href="/build?from={sentence.id}">Remix</a>
{#if showDiagram}
	<Diagram tree={sentence.tree} {grammar} {selectedPath} />
{/if}

<style>
	.actions {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-start;
		justify-content: center;
		gap: 0.5rem;
		margin-block: 1rem 1.5rem;
	}
</style>
