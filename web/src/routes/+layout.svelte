<script lang="ts">
	import '@fontsource-variable/atkinson-hyperlegible-next';
	import '../app.css';
	import { page } from '$app/state';
	import favicon from '#lib/assets/favicon.svg';

	let { children } = $props();

	// The stars page names itself in the header, so its own link would repeat it.
	const onStars = $derived(page.url.pathname === '/stars');
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<header>
	<div class="crumbs">
		<a class="title" href="/">RandSense</a>
		{#if onStars}
			<span class="divider" aria-hidden="true">/</span>
			<h1>Your stars</h1>
		{/if}
	</div>
	{#if !onStars}
		<nav aria-label="Main">
			<a class="button plain" href="/stars">Your stars</a>
		</nav>
	{/if}
</header>

<main>
	{@render children()}
</main>

<style>
	header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		inline-size: min(60rem, 100% - 2rem);
		margin-inline: auto;
		padding-block: 0.75rem;
	}

	.crumbs {
		display: flex;
		align-items: baseline;
		gap: 0.5rem;
		min-inline-size: 0;
	}

	.title {
		color: var(--title);
		font-size: var(--text-lg);
		font-weight: bold;
		text-decoration: none;
	}

	.divider {
		color: var(--separator);
		font-size: var(--text-lg);
	}

	h1 {
		margin: 0;
		font-size: var(--text-lg);
	}

	nav a {
		display: inline-flex;
		align-items: center;
		text-decoration: none;
	}

	main {
		inline-size: min(60rem, 100% - 2rem);
		margin-inline: auto;
		text-align: center;
	}
</style>
