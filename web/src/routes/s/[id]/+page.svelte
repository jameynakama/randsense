<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import SentenceView from '#lib/components/SentenceView.svelte';
	import { subscribe } from '#lib/stream.js';

	let { data } = $props();

	// Resets when the page moves to another sentence.
	let stars = $derived(data.sentence.star_count);

	onMount(() =>
		subscribe({
			onStars: (e) => {
				if (e.id === data.sentence.id) stars = e.count;
			}
		})
	);
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

<SentenceView sentence={data.sentence} grammar={data.grammar} bind:count={stars} />
