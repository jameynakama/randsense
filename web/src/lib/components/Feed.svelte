<script lang="ts">
	import { onMount } from 'svelte';
	import { flip } from 'svelte/animate';
	import { prefersReducedMotion } from 'svelte/motion';
	import { fly } from 'svelte/transition';
	import { FEED_SIZE, prepend } from '#lib/feed.js';
	import { subscribe, type StarsEvent } from '#lib/stream.js';
	import type { Sentence } from '#lib/types.js';
	import StarButton from './StarButton.svelte';

	let { initial, onstars }: { initial: Sentence[]; onstars?: (e: StarsEvent) => void } = $props();

	// The feed starts from the server's page, then the stream takes over.
	// svelte-ignore state_referenced_locally
	let feed = $state(initial);
	// What arrived while paused, newest first.
	let pending = $state<Sentence[]>([]);
	let paused = $state(false);
	const duration = $derived(prefersReducedMotion.current ? 0 : 300);
	const headingId = $props.id();

	onMount(() =>
		subscribe({
			onOpen: catchUp,
			onSentence: (s) => {
				if (paused) pending = prepend(pending, [s]);
				else feed = prepend(feed, [s]);
			},
			onStars: (e) => {
				for (const s of [...feed, ...pending]) if (s.id === e.id) s.star_count = e.count;
				onstars?.(e);
			}
		})
	);

	async function catchUp() {
		let latest: Sentence[];
		try {
			const res = await fetch(`/api/v1/sentences?limit=${FEED_SIZE}`);
			if (!res.ok) return;
			latest = await res.json();
		} catch {
			// The stream reconnects when the API is back, and that catches up.
			return;
		}
		if (!paused) {
			feed = latest;
			return;
		}
		const unseen = latest.filter((s) => !feed.some((f) => f.id === s.id));
		pending = prepend(pending, unseen);
	}

	function togglePause() {
		paused = !paused;
		if (!paused) {
			feed = prepend(feed, pending);
			pending = [];
		}
	}
</script>

<section class="feed" aria-labelledby={headingId}>
	<div class="head">
		<h2 id={headingId}>Latest sentences</h2>
		<button type="button" class="pill" aria-pressed={paused} onclick={togglePause}
			>Pause feed</button
		>
	</div>
	{#if pending.length}
		<p class="waiting">
			{pending.length}
			{pending.length === 1 ? 'new sentence' : 'new sentences'} waiting
		</p>
	{/if}
	<ul class="sentence-list">
		{#each feed as s (s.id)}
			<li animate:flip={{ duration }} in:fly={{ y: -16, duration }}>
				<a href="/s/{s.id}">{s.text}</a>
				<StarButton id={s.id} count={s.star_count} />
			</li>
		{/each}
	</ul>
</section>

<style>
	.feed {
		margin-block: 3rem;
	}

	.head {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: center;
		gap: 0.5rem 1.5rem;
	}

	.waiting {
		color: var(--outline);
	}
</style>
