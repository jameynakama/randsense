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
		line-height: 1.2;
		overflow-wrap: anywhere;
	}

	.word {
		position: relative;
		font: inherit;
		color: var(--word);
		background: none;
		border: none;
		padding: 0;
		margin: 0;
		cursor: pointer;
		border-radius: 0.15em;
		overflow-wrap: anywhere;
	}

	/* Stretches each word's target to 44px without spreading the lines or
	   the words apart. */
	.word::after {
		content: '';
		position: absolute;
		inset-block-start: 50%;
		inset-inline-start: 50%;
		inline-size: max(100%, 44px);
		block-size: max(100%, 44px);
		translate: -50% -50%;
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
