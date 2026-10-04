<script lang="ts">
	import StarButton from '#lib/components/StarButton.svelte';
	import { STARS_PAGE, starred } from '#lib/stars.js';

	let { data } = $props();

	// svelte-ignore state_referenced_locally
	let sentences = $state(data.sentences);
	// svelte-ignore state_referenced_locally
	let more = $state(data.sentences.length === STARS_PAGE);
	let loading = $state(false);
	let problem = $state('');

	async function showMore() {
		loading = true;
		problem = '';
		try {
			const next = await starred(fetch, sentences.length);
			sentences = [...sentences, ...next];
			more = next.length === STARS_PAGE;
		} catch {
			problem = 'Couldn’t load more. Try again.';
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Your stars · RandSense</title>
</svelte:head>

<h1>Your stars</h1>

{#if sentences.length}
	<ul class="sentence-list">
		{#each sentences as s (s.id)}
			<li>
				<a href="/s/{s.id}">{s.text}</a>
				<StarButton id={s.id} count={s.star_count} />
			</li>
		{/each}
	</ul>
	{#if more}
		<button type="button" class="pill" disabled={loading} onclick={showMore}>Show more</button>
	{/if}
{:else}
	<p>You haven’t starred any sentences in this browser yet.</p>
{/if}
<p class="problem" role="status">{problem}</p>

<style>
	.problem {
		color: var(--error);
	}
</style>
