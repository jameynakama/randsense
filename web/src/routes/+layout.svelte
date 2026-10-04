<script lang="ts">
	import '@fontsource-variable/atkinson-hyperlegible-next';
	import '../app.css';
	import { page } from '$app/state';
	import favicon from '#lib/assets/favicon.svg';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<header>
	<a class="title" href="/">RandSense</a>
	<nav aria-label="Main">
		<a href="/stars" aria-current={page.url.pathname === '/stars' ? 'page' : undefined}
			>Your stars</a
		>
	</nav>
</header>

<main>
	{@render children()}
</main>

<style>
	header {
		text-align: center;
		padding-block: 1rem;
	}

	.title {
		color: var(--title);
		font-size: 2.5rem;
		font-weight: bold;
		text-decoration: none;
	}

	nav a {
		display: inline-block;
		min-block-size: 44px;
		line-height: 44px;
		font-weight: bold;
		color: var(--outline);
	}

	nav a[aria-current='page'] {
		color: var(--accent);
	}

	main {
		inline-size: min(60rem, 100% - 2rem);
		margin-inline: auto;
		text-align: center;
	}
</style>
