<script lang="ts">
	import { tokens } from '#lib/tree.js';
	import type { Sentence } from '#lib/types.js';

	let {
		sentence,
		target = $bindable(null),
		onclose
	}: { sentence: Sentence; target?: number | null; onclose: () => void } = $props();

	// The API's limits on a comment, after trimming.
	const MIN = 10;
	const MAX = 1000;

	const words = $derived(tokens(sentence.tree).filter((t) => !t.punctuation));
	let comment = $state('');
	// Counted in code points, as the server counts.
	const length = $derived([...comment.trim()].length);
	let problem = $state('');
	let sending = $state(false);
	let sent = $state(false);
	const id = $props.id();
	let heading: HTMLHeadingElement | undefined = $state();

	$effect(() => {
		heading?.focus();
	});

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		if (length < MIN || length > MAX) {
			problem = 'Write 10 to 1,000 characters.';
			return;
		}
		problem = '';
		sending = true;
		try {
			const res = await fetch(`/api/v1/sentences/${sentence.id}/flags`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ comment: comment.trim(), word_index: target ?? undefined })
			});
			if (!res.ok) throw new Error(`status ${res.status}`);
			sent = true;
		} catch {
			problem = 'Couldn’t send your report. Try again.';
		} finally {
			sending = false;
		}
	}
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<section class="flag" aria-labelledby="{id}-heading">
	<h2 id="{id}-heading" tabindex="-1" bind:this={heading}>Report a problem</h2>
	<p role="status">{sent ? 'Thanks. Your report was sent.' : ''}</p>
	{#if sent}
		<button type="button" class="button" onclick={onclose}>Close</button>
	{:else}
		<form onsubmit={submit} novalidate>
			<label for="{id}-target">About</label>
			<select id="{id}-target" bind:value={target}>
				<option value={null}>The whole sentence</option>
				{#each words as w, n (w.index)}
					<option value={w.index}>{w.text} (word {n + 1})</option>
				{/each}
			</select>
			<label for="{id}-comment">What’s wrong?</label>
			<textarea
				id="{id}-comment"
				rows="4"
				bind:value={comment}
				aria-invalid={problem ? 'true' : undefined}
				aria-describedby="{id}-count {id}-problem"></textarea>
			<p id="{id}-count" class="hint">{length} of 1,000 characters, at least 10</p>
			<p id="{id}-problem" class="problem" aria-live="polite">{problem}</p>
			<div class="buttons">
				<button type="submit" class="button primary" disabled={sending}>Send</button>
				<button type="button" class="button plain" onclick={onclose}>Cancel</button>
			</div>
		</form>
	{/if}
</section>

<style>
	.flag {
		text-align: start;
		background: var(--fill);
		border-radius: 16px;
		padding: 1rem 1.25rem;
		max-inline-size: 40rem;
		margin: 1.5rem auto;
		overflow-wrap: anywhere;
	}

	h2 {
		margin-block-start: 0;
	}

	form {
		display: grid;
		gap: 0.5rem;
	}

	label {
		font-size: var(--text-sm);
		font-weight: 600;
		color: var(--secondary);
	}

	select,
	textarea {
		font: inherit;
		min-block-size: 44px;
		background: var(--bg);
		border: 1px solid var(--separator);
		border-radius: 10px;
		padding: 0.5rem 0.75rem;
		min-inline-size: 0;
	}

	.hint,
	.problem {
		margin: 0;
		font-size: var(--text-sm);
	}

	.hint {
		color: var(--secondary);
	}

	.buttons {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		margin-block-start: 0.5rem;
	}
</style>
