import { describe, expect, it } from 'vitest';
import { ARMED_KEYS, CTRL_COMBOS, armedLabel, comboTitle, modifierChord } from '$lib/keymap';

describe('terminal key pad', () => {
  it('carries the combinations a phone cannot otherwise produce', () => {
    const keys = CTRL_COMBOS.map((combo) => combo.key);
    // The four the manager asked for by name, because a plan approval asked for
    // Ctrl+A and the pad had no Ctrl at all.
    expect(keys).toContain('a');
    expect(keys).toContain('c');
    expect(keys).toContain('d');
    expect(keys).toContain('b');
    for (const combo of CTRL_COMBOS) {
      expect(combo.title).toMatch(/^Ctrl\+\w/);
      expect(combo.label).not.toBe('');
    }
  });

  it('sends a combination whole, with no modifier armed', () => {
    expect(modifierChord({ ctrl: false, alt: false, shift: false }, 'c')).toBeNull();
    expect(comboTitle('c')).toBe('Ctrl+C');
  });

  it('builds the chord herdr parses when a modifier is latched', () => {
    expect(modifierChord({ ctrl: true, alt: false, shift: false }, 'a'))
      .toEqual({ chord: 'ctrl+a', label: 'Ctrl+A' });
    expect(modifierChord({ ctrl: true, alt: true, shift: true }, 'c'))
      .toEqual({ chord: 'ctrl+alt+shift+c', label: 'Ctrl+Alt+Shift+C' });
    expect(modifierChord({ ctrl: true, alt: false, shift: false }, 'Tab'))
      .toEqual({ chord: 'ctrl+tab', label: 'Ctrl+Tab' });
    expect(modifierChord({ ctrl: false, alt: false, shift: true }, 'ArrowUp'))
      .toEqual({ chord: 'shift+arrowup', label: 'Shift+Arrowup' });
  });

  it('names the latched modifiers in the order they are applied', () => {
    expect(armedLabel({ ctrl: true, alt: true, shift: true })).toBe('Ctrl+Alt+Shift');
    expect(armedLabel({ ctrl: false, alt: true, shift: false })).toBe('Alt');
    expect(armedLabel({ ctrl: false, alt: false, shift: false })).toBe('');
  });

  it('offers the keys a prompt needs without a keyboard', () => {
    const labels = ARMED_KEYS.map((key) => key.label);
    expect(labels).toContain('Esc');
    expect(labels).toContain('Tab');
    expect(labels).toContain('Enter');
    expect(labels).toContain('↑');
    expect(labels).toContain('↓');
  });
});
