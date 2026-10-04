<script lang="ts">
	import { isStarred, setStarred, voterToken } from '#lib/voter.js';

	let { id, count = $bindable() }: { id: string; count: number } = $props();

	// Read from storage after hydration, so the server's HTML (never starred)
	// matches what the browser hydrates.
	// eslint-disable-next-line svelte/prefer-writable-derived
	let starred = $state(false);
	let busy = $state(false);
	let problem = $state('');

	$effect(() => {
		starred = isStarred(id);
	});

	async function toggle() {
		busy = true;
		problem = '';
		try {
			const res = await fetch(`/api/v1/sentences/${id}/stars`, {
				method: starred ? 'DELETE' : 'POST',
				headers: { 'X-Voter': voterToken() }
			});
			if (!res.ok) throw new Error(`status ${res.status}`);
			count = ((await res.json()) as { count: number }).count;
			starred = !starred;
			setStarred(id, starred);
		} catch {
			problem = starred
				? 'Couldn’t remove your star. Try again.'
				: 'Couldn’t save your star. Try again.';
		} finally {
			busy = false;
		}
	}
</script>

<span class="star">
	<button
		type="button"
		class="pill"
		aria-pressed={starred}
		aria-label="Star, {count} {count === 1 ? 'star' : 'stars'}"
		disabled={busy}
		onclick={toggle}><span aria-hidden="true">{starred ? '★' : '☆'}</span> {count}</button
	>
	<span role="status" class="problem">{problem}</span>
</span>

<style>
	.problem {
		display: block;
		color: var(--error);
	}

	.problem:empty {
		display: none;
	}
</style>
