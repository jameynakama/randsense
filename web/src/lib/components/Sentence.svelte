<script lang="ts" module>
	export function wordId(sentenceId: string, index: number): string {
		return `word-${sentenceId}-${index}`;
	}
</script>

<script lang="ts">
	import { tokens } from '#lib/tree.js';
	import type { Sentence } from '#lib/types.js';

	let { sentence, selected = $bindable(null) }: { sentence: Sentence; selected?: number | null } =
		$props();

	const words = $derived(tokens(sentence.tree));
</script>

<!-- eslint-disable svelte/no-useless-mustaches -- {' '} keeps the space between words, which a bare space beside a tag doesn't reliably do -->
<p class="sentence" role="group" aria-label={sentence.text}>
	{#each words as t (t.index)}
		{#if t.punctuation}<span class="punct">{t.text}</span>{:else}{#if t.index > 0}{' '}{/if}<button
				type="button"
				id={wordId(sentence.id, t.index)}
				class="word"
				aria-pressed={selected === t.index}
				onclick={() => (selected = selected === t.index ? null : t.index)}>{t.text}</button
			>{/if}
	{/each}<span class="punct">.</span>
</p>

<style>
	.sentence {
		margin: 0;
		font-size: clamp(1.5rem, 8vw, 4rem);
		line-height: 1.3;
		overflow-wrap: anywhere;
	}

	.word {
		font: inherit;
		color: var(--word);
		background: none;
		border: none;
		padding: 0.1em 0.15em;
		margin: 0;
		min-block-size: 44px;
		min-inline-size: 44px;
		cursor: pointer;
		border-radius: 0.2em;
		overflow-wrap: anywhere;
	}

	.word:hover,
	.word[aria-pressed='true'] {
		color: var(--word-selected);
	}

	.word[aria-pressed='true'] {
		text-decoration: underline;
		text-underline-offset: 0.15em;
	}

	.punct {
		color: var(--word);
	}
</style>
