import { get } from 'svelte/store';
import { render, screen } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import TerminalView from '$components/TerminalView.svelte';
import { TERMINAL_KEY_CONTROLS_KEY } from '$lib/config';
import { setTerminalKeyControls, terminalKeyControls } from '$lib/preferences';
import { relayStore } from '$lib/store';
import type { Agent } from '$lib/types';

function agent(): Agent {
  return {
    relay_id: 'fedora',
    relay_label: 'Fedora',
    raw_pane_id: 'w1:p1',
    pane_id: 'fedora::w1:p1',
    project: 'relay',
    agent: 'codex',
    status: 'working',
    cwd: '/home/test/relay',
  };
}

function mountTerminal() {
  const current = agent();
  vi.spyOn(relayStore, 'readPane').mockImplementation(() => undefined);
  vi.spyOn(relayStore, 'loadSlashCommands').mockResolvedValue({ commands: [], truncated: false });
  return render(TerminalView, {
    agent: current,
    allAgents: [current],
    frame: { paneId: current.pane_id, content: 'ready', format: 'plain' },
    responding: new Set<string>(),
  });
}

beforeEach(() => {
  setTerminalKeyControls(true);
});

afterEach(() => {
  setTerminalKeyControls(true);
  vi.restoreAllMocks();
});

describe('terminal key controls', () => {
  it('hides the key row from the floating toggle and brings it back', async () => {
    const user = userEvent.setup();
    mountTerminal();

    expect(screen.getByRole('button', { name: 'Enter' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Hide key controls' })).toBeVisible();

    await user.click(screen.getByRole('button', { name: 'Hide key controls' }));
    expect(screen.queryByRole('button', { name: 'Enter' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Show key controls' })).toBeVisible();

    await user.click(screen.getByRole('button', { name: 'Show key controls' }));
    expect(screen.getByRole('button', { name: 'Enter' })).toBeInTheDocument();
  });

  it('keeps the prompt composer available while the key row is hidden', async () => {
    const user = userEvent.setup();
    mountTerminal();

    await user.click(screen.getByRole('button', { name: 'Hide key controls' }));

    expect(screen.getByRole('combobox', { name: 'Prompt' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Send prompt' })).toBeInTheDocument();
  });

  it('remembers the hidden choice and reports it back through the store', () => {
    setTerminalKeyControls(false);
    expect(get(terminalKeyControls)).toBe(false);
    expect(localStorage.getItem(TERMINAL_KEY_CONTROLS_KEY)).toBe('hidden');

    setTerminalKeyControls(true);
    expect(get(terminalKeyControls)).toBe(true);
    expect(localStorage.getItem(TERMINAL_KEY_CONTROLS_KEY)).toBe('shown');
  });
});
