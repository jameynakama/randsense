<script lang="ts">
	import '@fontsource-variable/atkinson-hyperlegible-next';
	import '../app.css';
	import favicon from '#lib/assets/favicon.svg';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<header>
	<a class="title" href="/">RandSense</a>
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

	main {
		inline-size: min(60rem, 100% - 2rem);
		margin-inline: auto;
		text-align: center;
	}
</style>
