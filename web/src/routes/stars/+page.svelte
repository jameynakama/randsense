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
			// Starring or unstarring elsewhere shifts the pages, so the next one
			// can repeat a sentence already shown.
			const fresh = next.filter((s) => !sentences.some((have) => have.id === s.id));
			sentences = [...sentences, ...fresh];
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
		<button type="button" class="button" disabled={loading} onclick={showMore}>Show more</button>
	{/if}
{:else}
	<p class="empty">You haven’t starred any sentences in this browser yet.</p>
{/if}
<p class="problem" role="status">{problem}</p>

<style>
	.empty {
		color: var(--secondary);
	}
</style>
