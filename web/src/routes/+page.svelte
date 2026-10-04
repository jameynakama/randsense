<script lang="ts">
	import { onMount } from 'svelte';
	import Feed from '#lib/components/Feed.svelte';
	import SentenceView from '#lib/components/SentenceView.svelte';
	import type { Sentence } from '#lib/types.js';

	let { data } = $props();

	let current = $state<Sentence | null>(null);
	let generating = $state(false);
	let problem = $state('');

	async function generate() {
		generating = true;
		problem = '';
		try {
			const res = await fetch('/api/v1/sentences/random');
			if (!res.ok) throw new Error(`status ${res.status}`);
			current = await res.json();
		} catch {
			problem = 'Couldn’t make a sentence. Try again.';
		} finally {
			generating = false;
		}
	}

	// Each visit starts with a fresh sentence. Generating here, in the
	// browser, keeps link previews and crawlers from filling the feed.
	onMount(generate);
</script>

<svelte:head>
	<title>RandSense</title>
	<meta name="description" content="Grammatically sound, randomly generated nonsense." />
</svelte:head>

<h1 class="visually-hidden">Random sentences</h1>

<button type="button" class="pill" disabled={generating} onclick={generate}>Generate</button>
<p class="problem" role="status">{problem}</p>
<!-- Announces the visitor's own sentence. The feed is never read aloud. -->
<p class="visually-hidden" aria-live="polite">{current?.text ?? ''}</p>

{#if current}
	<SentenceView sentence={current} bind:count={current.star_count} />
{/if}

<Feed
	initial={data.sentences}
	onstars={(e) => {
		if (current?.id === e.id) current.star_count = e.count;
	}}
/>

<style>
	.problem {
		color: var(--error);
	}
</style>
