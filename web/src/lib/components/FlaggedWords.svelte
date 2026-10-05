<script lang="ts">
	import { wordKey } from '#lib/admin.js';
	import type { FlaggedWord } from '#lib/types.js';

	let { words }: { words: FlaggedWord[] } = $props();
</script>

{#if words.length}
	<table>
		<thead>
			<tr>
				<th scope="col">Word</th>
				<th scope="col">Part of speech</th>
				<th scope="col">Flags</th>
			</tr>
		</thead>
		<tbody>
			{#each words as w (wordKey(w))}
				<tr>
					<td>{w.lemma}</td>
					<td>{w.pos.toLowerCase()}</td>
					<td>{w.count}</td>
				</tr>
			{/each}
		</tbody>
	</table>
{:else}
	<p class="empty">No words have been flagged yet.</p>
{/if}

<style>
	table {
		inline-size: 100%;
		margin-block: 1rem;
		border-collapse: collapse;
		text-align: start;
	}

	th,
	td {
		padding: 0.5rem 0.75rem;
		text-align: start;
		border-block-end: 1px solid var(--separator);
		overflow-wrap: anywhere;
	}

	th {
		color: var(--secondary);
		font-size: var(--text-sm);
	}

	/* Counts line up on the right, where numbers compare best. */
	th:last-child,
	td:last-child {
		text-align: end;
	}

	.empty {
		color: var(--secondary);
	}
</style>
