<script lang="ts">
  // Two empty postures, both primary screens (design §7):
  //  - "no-sources": a pre-flight diagnostic checklist that teaches the path
  //    model with a live ✓/✗ per agent — calm, never a sad shrug.
  //  - "no-range": "No usage in this range" with a one-tap jump chip.
  import { SOURCE_META, SOURCE_ORDER } from '../lib/palette';

  let {
    mode,
    onJump,
  }: { mode: 'no-sources' | 'no-range'; onJump?: () => void } = $props();

  const DEFAULT_PATHS: Record<string, string> = {
    claude: '~/.claude/projects',
    codex: '~/.codex/sessions',
    opencode: '~/.local/share/opencode',
    amp: '~/.local/share/amp',
    pi: '~/.pi/agent/sessions',
  };
  const ENV_VARS: Record<string, string> = {
    claude: 'CLAUDE_CONFIG_DIR',
    codex: 'CODEX_HOME',
    opencode: 'OPENCODE_DATA_DIR',
    amp: 'AMP_DATA_DIR',
    pi: 'PI_AGENT_DIR',
  };
  // In mock/no-sources we present a believable pre-flight where 3 of 5 are found.
  const FOUND: Record<string, boolean> = { claude: true, codex: true, opencode: true, amp: false, pi: false };
</script>

{#if mode === 'no-range'}
  <div class="empty">
    <div class="art" aria-hidden="true">∿</div>
    <h2>No usage in this range</h2>
    <p>The sources are readable — there's just nothing logged for the selected window.</p>
    {#if onJump}
      <button class="jump" onclick={onJump}>Jump to last 30 days →</button>
    {/if}
  </div>
{:else}
  <div class="diag">
    <div class="diag-head">
      <h2>Pre-flight: where's your data?</h2>
      <p>llmut reads the same local log files the CLI reads. Nothing leaves this machine.</p>
    </div>
    <ul class="checklist">
      {#each SOURCE_ORDER as id (id)}
        <li class:found={FOUND[id]}>
          <span class="glyph" aria-hidden="true">{SOURCE_META[id].glyph}</span>
          <span class="name">{SOURCE_META[id].label}</span>
          <code class="path mono">{DEFAULT_PATHS[id]}</code>
          <code class="env mono">${ENV_VARS[id]}</code>
          {#if FOUND[id]}
            <span class="dot ok" title="Found & readable">✓</span>
          {:else}
            <span class="dot miss" title="Not found">✗</span>
            <button class="pick">Point us at it →</button>
          {/if}
        </li>
      {/each}
    </ul>
  </div>
{/if}

<style>
  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 10px;
    text-align: center;
    padding: 72px 24px;
    border: 1px solid var(--hairline);
    border-radius: var(--r-card);
    background: var(--surface);
  }
  .art {
    font-size: 44px;
    color: var(--amber);
    opacity: 0.7;
  }
  .empty h2 {
    font-size: 18px;
    font-weight: 600;
  }
  .empty p {
    color: var(--text-muted);
    max-width: 46ch;
  }
  .jump {
    margin-top: 8px;
    padding: 8px 16px;
    border: 1px solid color-mix(in srgb, var(--amber) 45%, var(--hairline));
    border-radius: var(--r-chip);
    background: color-mix(in srgb, var(--amber) 10%, transparent);
    color: var(--amber);
    font-size: 13px;
    font-weight: 500;
  }
  .jump:hover {
    background: color-mix(in srgb, var(--amber) 18%, transparent);
  }

  .diag {
    border: 1px solid var(--hairline);
    border-radius: var(--r-card);
    background: var(--surface);
    padding: 24px;
  }
  .diag-head h2 {
    font-size: 18px;
    font-weight: 600;
    margin-bottom: 4px;
  }
  .diag-head p {
    color: var(--text-muted);
    margin-bottom: 18px;
  }
  .checklist {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    border: 1px solid var(--hairline);
    border-radius: var(--r-card);
    overflow: hidden;
  }
  .checklist li {
    display: grid;
    grid-template-columns: 18px 96px 1fr auto auto auto;
    align-items: center;
    gap: 14px;
    padding: 11px 14px;
    border-bottom: 1px solid var(--hairline);
  }
  .checklist li:last-child {
    border-bottom: 0;
  }
  .glyph {
    color: var(--text-muted);
    text-align: center;
  }
  .name {
    font-weight: 500;
  }
  .path {
    font-size: 12px;
    color: var(--text-muted);
  }
  .env {
    font-size: 11px;
    color: var(--text-faint);
  }
  .dot {
    font-size: 13px;
    width: 18px;
    text-align: center;
  }
  .dot.ok {
    color: var(--teal);
  }
  .dot.miss {
    color: var(--caution);
  }
  .pick {
    background: transparent;
    border: 1px solid var(--hairline-strong);
    border-radius: var(--r-chip);
    color: var(--text-muted);
    font-size: 12px;
    padding: 4px 10px;
  }
  .pick:hover {
    color: var(--text);
    border-color: var(--amber);
  }
</style>
