<script lang="ts">
	import { isStarred, setStarred, voterToken } from '#lib/voter.js';

	let { id, count = $bindable() }: { id: string; count: number } = $props();

	// Read from storage after hydration, so the server's HTML (never starred)
	// matches what the browser hydrates.
	let starred = $state(false);
	let busy = $state(false);
	let problem = $state('');

	$effect(() => {
		starred = isStarred(id);
		problem = '';
	});

	async function toggle() {
		if (busy) return;
		// The page can show another sentence before the answer arrives, so the
		// answer is applied to the sentence it was for.
		const target = id;
		const starring = !starred;
		busy = true;
		problem = '';
		try {
			const res = await fetch(`/api/v1/sentences/${target}/stars`, {
				method: starring ? 'POST' : 'DELETE',
				headers: { 'X-Voter': voterToken() }
			});
			if (!res.ok) throw new Error(`status ${res.status}`);
			const answer = ((await res.json()) as { count: number }).count;
			setStarred(target, starring);
			if (id === target) {
				count = answer;
				starred = starring;
			}
		} catch {
			if (id === target) {
				problem = starring
					? 'Couldn’t save your star. Try again.'
					: 'Couldn’t remove your star. Try again.';
			}
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
</style>
