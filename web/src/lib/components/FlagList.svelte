<script lang="ts">
	import { flaggedWord } from '#lib/admin.js';
	import type { Flag } from '#lib/types.js';

	let { flags }: { flags: Flag[] } = $props();

	const when = (iso: string) =>
		new Date(iso).toLocaleString('en-US', { dateStyle: 'medium', timeStyle: 'short' });
</script>

{#if flags.length}
	<ul class="flags">
		{#each flags as f (f.id)}
			{@const word = flaggedWord(f)}
			<li>
				<a href="/s/{f.sentence.id}">{f.sentence.text}</a>
				<p class="about">
					{#if word}
						About “{word}” ({f.lemma}, {f.pos?.toLowerCase()})
					{:else}
						About the whole sentence
					{/if}
				</p>
				<p class="comment">{f.comment}</p>
				<time datetime={f.created_at}>{when(f.created_at)}</time>
			</li>
		{/each}
	</ul>
{:else}
	<p class="empty">No one has flagged anything yet.</p>
{/if}

<style>
	.flags {
		padding: 0;
		margin-block: 1rem;
		list-style: none;
		text-align: start;
		background: var(--fill);
		border-radius: 12px;
	}

	li {
		display: grid;
		gap: 0.25rem;
		padding: 0.75rem 1rem;
		border-block-end: 1px solid var(--separator);
		overflow-wrap: anywhere;
	}

	li:last-child {
		border-block-end: none;
	}

	a {
		color: var(--text);
		font-weight: 600;
	}

	p {
		margin: 0;
	}

	.about,
	time,
	.empty {
		color: var(--secondary);
		font-size: var(--text-sm);
	}

	/* Keeps the line breaks people typed. */
	.comment {
		white-space: pre-line;
	}
</style>
