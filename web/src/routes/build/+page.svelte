<script lang="ts">
	import { tick } from 'svelte';
	import { goto } from '$app/navigation';
	import Diagram from '#lib/components/Diagram.svelte';
	import ExpansionSheet from '#lib/components/ExpansionSheet.svelte';
	import {
		choose,
		clear,
		draftText,
		fill,
		nodeAt,
		remix,
		start,
		toggleLock,
		undo,
		type Builder
	} from '#lib/builder.js';
	import { key } from '#lib/diagram.js';
	import { leaves } from '#lib/tree.js';
	import type { Sentence } from '#lib/types.js';

	let { data } = $props();

	// Resets when the page moves to another sentence or to a fresh start.
	// Builder functions return new state, and structuredClone can't copy a
	// deep $state proxy; a derived isn't one.
	let b = $derived<Builder>(data.from ? remix(data.from.tree) : start(data.grammar));
	let stale = $state(false);
	// The phrase whose sheet is open.
	let open = $state<number[] | null>(null);
	let busy = $state(false);
	let problem = $state('');
	let problemPath = $state<number[] | null>(null);

	const text = $derived(draftText(b.draft.tree));
	const openNode = $derived(open && nodeAt(b.draft.tree, open));

	// apply shows the next state, closes the sheet and any problem, and
	// returns focus to the phrase whose sheet was open.
	async function apply(next: Builder) {
		const path = open;
		b = next;
		open = null;
		stale = false;
		problem = '';
		problemPath = null;
		if (!path) return;
		await tick();
		document.getElementById(`slot-${key(path)}`)?.focus();
	}

	async function realize() {
		busy = true;
		stale = false;
		problem = '';
		problemPath = null;
		const posted = b;
		try {
			const res = await fetch('/api/v1/sentences/realize', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(posted.draft.tree)
			});
			if (!res.ok && res.status !== 422 && res.status !== 400)
				throw new Error(`status ${res.status}`);
			const body = await res.json();
			// The diagram stays editable during the request. Edits made meanwhile
			// win, and a response for the older draft would index the wrong tree.
			if (b !== posted) return;
			if (res.status === 400) {
				stale = true;
				return;
			}
			if (res.status === 422) {
				const target = leaves(posted.draft.tree)[(body as { leaf: number }).leaf];
				problemPath = target.path;
				problem = target.node.locked
					? 'No word fits here with that word locked. Unlock it, or change the phrase above it.'
					: 'No word fits this slot right now. Change the phrase above it.';
				return;
			}
			b = fill(b, body);
		} catch {
			problem = 'Couldn’t fill the sentence. Try again.';
		} finally {
			busy = false;
		}
	}

	async function keep() {
		busy = true;
		problem = '';
		try {
			const { tree, signature } = b.draft.filled!;
			const res = await fetch('/api/v1/sentences', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ tree, signature })
			});
			if (res.status === 400) {
				problem = 'This one waited too long to be kept. Reroll, then keep.';
				return;
			}
			if (!res.ok) throw new Error(`status ${res.status}`);
			const kept: Sentence = await res.json();
			await goto(`/s/${kept.id}`);
		} catch {
			problem = 'Couldn’t keep the sentence. Try again.';
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>Build a sentence · RandSense</title>
	<meta name="description" content="Build a grammatically sound sentence from the grammar down." />
</svelte:head>

<p class="intro">
	Tap a dashed slot to choose what goes in it, or leave it for Fill to choose. After a fill, tap a
	word to lock it, then reroll the rest.
</p>

<div class="actions">
	<button type="button" class="button primary" disabled={busy} onclick={realize}
		>{text ? 'Reroll' : 'Fill'}</button
	>
	<button
		type="button"
		class="button plain"
		disabled={!b.undo.length}
		onclick={() => apply(undo(b))}>Undo</button
	>
	<button type="button" class="button" disabled={busy || !b.draft.filled} onclick={keep}
		>Keep this one</button
	>
</div>
<p class="problem" role="status">
	{#if stale}
		This sentence’s structure isn’t one the grammar makes anymore. <a href="/build">Start over</a>
	{:else}
		{problem}
	{/if}
</p>
<p class="draft" aria-live="polite">{text}</p>

<Diagram
	tree={b.draft.tree}
	grammar={data.grammar}
	editing={{
		onphrase: (path) => (open = path),
		onword: (path) => apply(toggleLock(b, path)),
		problemPath
	}}
/>

{#if open && openNode}
	<ExpansionSheet
		symbol={openNode.symbol}
		grammar={data.grammar}
		filled={!!openNode.children?.length}
		onchoose={(rule) => apply(choose(b, open!, rule))}
		onclear={() => apply(clear(b, open!))}
		onclose={() => apply(b)}
	/>
{/if}

<style>
	.intro {
		color: var(--secondary);
		max-inline-size: 40rem;
		margin-inline: auto;
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		justify-content: center;
		gap: 0.5rem;
	}

	.draft {
		font-size: var(--text-xl);
		overflow-wrap: anywhere;
	}
</style>
