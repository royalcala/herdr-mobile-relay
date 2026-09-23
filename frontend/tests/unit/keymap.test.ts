import { describe, expect, it } from 'vitest';
import { armedLabel, modifierChord, type Modifiers } from '$lib/keymap';

/** The chord the pad builds for a key with those modifiers armed. */
function chordWith(modifiers: Modifiers, key: string): string {
  const built = modifierChord(modifiers, key);
  if (!built) throw new Error(`no chord for ${key}`);
  return built.chord;
}

describe('latched terminal modifiers', () => {
  it('carries an armed Ctrl on the next key, exactly like Alt', () => {
    const ctrl = { ctrl: true, alt: false, shift: false };
    expect(modifierChord(ctrl, 'c')).toEqual({ chord: 'ctrl+c', label: 'Ctrl+C' });
    // The combinations a person reaches for when approving a plan or
    // interrupting an agent.
    expect(chordWith(ctrl, 'a')).toBe('ctrl+a');
    expect(chordWith(ctrl, 'b')).toBe('ctrl+b');
    expect(chordWith(ctrl, 'd')).toBe('ctrl+d');
    // Keys from the app's own pad, not the phone keyboard.
    expect(modifierChord(ctrl, 'Tab')).toEqual({ chord: 'ctrl+tab', label: 'Ctrl+Tab' });
    expect(chordWith(ctrl, 'Escape')).toBe('ctrl+escape');
    expect(chordWith(ctrl, 'ArrowUp')).toBe('ctrl+arrowup');
  });

  it('stacks the latched modifiers in the order they apply', () => {
    expect(modifierChord({ ctrl: true, alt: false, shift: true }, 'Tab'))
      .toEqual({ chord: 'ctrl+shift+tab', label: 'Ctrl+Shift+Tab' });
    expect(modifierChord({ ctrl: false, alt: true, shift: false }, 'Enter'))
      .toEqual({ chord: 'alt+enter', label: 'Alt+Enter' });
  });

  it('sends a key on its own when nothing is armed', () => {
    expect(modifierChord({ ctrl: false, alt: false, shift: false }, 'Tab')).toBeNull();
  });

  it('names what is armed, so the latch is visible', () => {
    expect(armedLabel({ ctrl: true, alt: false, shift: false })).toBe('Ctrl');
    expect(armedLabel({ ctrl: true, alt: true, shift: true })).toBe('Ctrl+Alt+Shift');
    expect(armedLabel({ ctrl: false, alt: false, shift: false })).toBe('');
  });
});
