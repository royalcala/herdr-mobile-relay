/**
 * The terminal key pad's latched modifiers, kept out of the component so the
 * mechanism can be tested on its own.
 *
 * A phone has no Ctrl key, so Ctrl, Alt and Shift are buttons that latch: press
 * one, it stays visibly armed, and the next key pressed on the app's own pad
 * carries it — Ctrl then Tab sends `ctrl+tab`. The latch is for exactly one key
 * and releases as the chord goes out.
 */

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
