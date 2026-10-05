<script lang="ts">
	import { goto } from '$app/navigation';
	import FlagList from '#lib/components/FlagList.svelte';
	import FlaggedWords from '#lib/components/FlaggedWords.svelte';
	import Tabs from '#lib/components/Tabs.svelte';
	import {
		ADMIN_PAGE,
		LoggedOut,
		appendNew,
		flaggedWords,
		flags,
		logout,
		wordKey
	} from '#lib/admin.js';

	let { data } = $props();

	// svelte-ignore state_referenced_locally
	let flagList = $state(data.flags);
	// svelte-ignore state_referenced_locally
	let moreFlags = $state(data.flags.length === ADMIN_PAGE);
	// How far each list has read. New flags push rows down, so a page can
	// repeat rows already shown, and the shown count would fall behind.
	// svelte-ignore state_referenced_locally
	let flagOffset = $state(data.flags.length);
	// svelte-ignore state_referenced_locally
	let words = $state(data.words);
	// svelte-ignore state_referenced_locally
	let moreWords = $state(data.words.length === ADMIN_PAGE);
	// svelte-ignore state_referenced_locally
	let wordOffset = $state(data.words.length);
	let loading = $state(false);
	let problem = $state('');

	// run makes an admin request, sending the admin to log in again if the
	// session has ended.
	async function run(request: () => Promise<void>, failure: string) {
		loading = true;
		problem = '';
		try {
			await request();
		} catch (e) {
			if (e instanceof LoggedOut) await goto('/admin/login');
			else problem = failure;
		} finally {
			loading = false;
		}
	}

	const showMoreFlags = () =>
		run(async () => {
			const next = await flags(fetch, flagOffset);
			flagOffset += next.length;
			flagList = appendNew(flagList, next, (f) => f.id);
			moreFlags = next.length === ADMIN_PAGE;
		}, 'Couldn’t load more. Try again.');

	const showMoreWords = () =>
		run(async () => {
			const next = await flaggedWords(fetch, wordOffset);
			wordOffset += next.length;
			words = appendNew(words, next, wordKey);
			moreWords = next.length === ADMIN_PAGE;
		}, 'Couldn’t load more. Try again.');

	const leave = () =>
		run(async () => {
			await logout(fetch);
			await goto('/admin/login');
		}, 'Couldn’t log out. Try again.');
</script>

<svelte:head>
	<title>Flags · RandSense</title>
</svelte:head>

<div class="bar">
	<button type="button" class="button plain" disabled={loading} onclick={leave}>Log out</button>
</div>

<Tabs
	label="Flags"
	tabs={[
		{ id: 'flags', label: 'Newest flags' },
		{ id: 'words', label: 'Most-flagged words' }
	]}
>
	{#snippet panel(id)}
		{#if id === 'flags'}
			<FlagList flags={flagList} />
			{#if moreFlags}
				<button type="button" class="button" disabled={loading} onclick={showMoreFlags}
					>Show more</button
				>
			{/if}
		{:else}
			<FlaggedWords {words} />
			{#if moreWords}
				<button type="button" class="button" disabled={loading} onclick={showMoreWords}
					>Show more</button
				>
			{/if}
		{/if}
	{/snippet}
</Tabs>
<p class="problem" role="status">{problem}</p>

<style>
	.bar {
		display: flex;
		justify-content: flex-end;
	}
</style>
