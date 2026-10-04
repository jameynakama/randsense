<script lang="ts">
	import { tick } from 'svelte';
	import { page } from '$app/state';
	import Sentence, { wordId } from '#lib/components/Sentence.svelte';
	import StarButton from '#lib/components/StarButton.svelte';
	import Structure from '#lib/components/Structure.svelte';
	import WordCard from '#lib/components/WordCard.svelte';
	import { leaves } from '#lib/tree.js';

	let { data } = $props();

	// Both reset when the page moves to another sentence.
	let selected = $derived.by<number | null>(() => {
		void data.sentence.id;
		return null;
	});
	let stars = $derived(data.sentence.star_count);
	const selectedPath = $derived(
		selected === null ? null : leaves(data.sentence.tree)[selected].path
	);

	async function closeCard() {
		const opener = selected;
		selected = null;
		await tick();
		if (opener !== null) document.getElementById(wordId(data.sentence.id, opener))?.focus();
	}
</script>

<svelte:head>
	<title>{data.sentence.text} · RandSense</title>
	<meta name="description" content="A grammatically sound, randomly generated sentence." />
	<meta property="og:site_name" content="RandSense" />
	<meta property="og:type" content="article" />
	<meta property="og:title" content={data.sentence.text} />
	<meta property="og:url" content={page.url.href} />
	<meta name="twitter:card" content="summary" />
</svelte:head>

<h1 class="visually-hidden">A random sentence</h1>

<Sentence sentence={data.sentence} bind:selected />

<div class="actions">
	<StarButton id={data.sentence.id} bind:count={stars} />
</div>

{#if selected !== null}
	<WordCard tree={data.sentence.tree} index={selected} onclose={closeCard} />
{/if}

<Structure tree={data.sentence.tree} {selectedPath} open />

<style>
	.actions {
		margin-block: 1.5rem;
	}
</style>
