/**
 * The terminal key pad's vocabulary, kept out of the component so it can be
 * tested on its own.
 *
 * Two different things live here:
 *
 *  - one-tap combinations (^C, ^D, ^B, ^A…), which is what a phone can actually
 *    press. A phone has no Ctrl key, and arming a modifier and then typing a
 *    letter into a hidden input is not something anyone should have to do to
 *    interrupt an agent;
 *  - the chord builder behind the latched modifiers, for everything the pad
 *    does not cover: arm Ctrl (or Alt, or Shift), press one key, and the chord
 *    goes out once and the latch releases.
 */

export interface KeyCombo {
  /** The letter the combination carries, lowercase. */
  key: string;
  /** Compact pad label. */
  label: string;
  /** Spoken and hovered name. */
  title: string;
}

/** Combinations that agents and terminal multiplexers actually ask for. */
export const CTRL_COMBOS: KeyCombo[] = [
  { key: 'a', label: '^A', title: 'Ctrl+A' },
  { key: 'b', label: '^B', title: 'Ctrl+B' },
  { key: 'c', label: '^C', title: 'Ctrl+C' },
  { key: 'd', label: '^D', title: 'Ctrl+D' },
];

/** Keys the pad sends directly, without a hidden input. */
export const ARMED_KEYS: KeyCombo[] = [
  { key: 'Escape', label: 'Esc', title: 'Escape' },
  { key: 'Tab', label: 'Tab', title: 'Tab' },
  { key: 'Enter', label: 'Enter', title: 'Enter' },
  { key: 'ArrowUp', label: '↑', title: 'Arrow up' },
  { key: 'ArrowDown', label: '↓', title: 'Arrow down' },
  { key: 'ArrowLeft', label: '←', title: 'Arrow left' },
  { key: 'ArrowRight', label: '→', title: 'Arrow right' },
];

export interface Modifiers {
  ctrl: boolean;
  alt: boolean;
  shift: boolean;
}

export function armedLabel(modifiers: Modifiers): string {
  return [
    modifiers.ctrl ? 'Ctrl' : '',
    modifiers.alt ? 'Alt' : '',
    modifiers.shift ? 'Shift' : '',
  ].filter(Boolean).join('+');
}

/**
 * The chord herdr expects, e.g. `ctrl+c`, or null when nothing is armed and the
 * key should go out on its own.
 */
export function modifierChord(
  modifiers: Modifiers,
  key: string,
): { chord: string; label: string } | null {
  const parts: string[] = [];
  const labels: string[] = [];
  if (modifiers.ctrl) { parts.push('ctrl'); labels.push('Ctrl'); }
  if (modifiers.alt) { parts.push('alt'); labels.push('Alt'); }
  if (modifiers.shift) { parts.push('shift'); labels.push('Shift'); }
  if (!parts.length) return null;
  parts.push(key.toLocaleLowerCase());
  labels.push(key.length === 1
    ? key.toLocaleUpperCase()
    : key[0].toLocaleUpperCase() + key.slice(1).toLocaleLowerCase());
  return { chord: parts.join('+'), label: labels.join('+') };
}

/** The keys a combination turns into, for its activity label. */
export function comboTitle(key: string): string {
  const combo = CTRL_COMBOS.find((candidate) => candidate.key === key);
  return combo ? combo.title : `Ctrl+${key.toLocaleUpperCase()}`;
}
