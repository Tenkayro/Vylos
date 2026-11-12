import { writable } from 'svelte/store';
import type { BootstrapState } from '../services/api';

export type NavSection =
  | 'overview'
  | 'devices'
  | 'activity'
  | 'screen_time'
  | 'applications'
  | 'websites'
  | 'protection'
  | 'settings';

export const activeSection = writable<NavSection>('overview');
export const selectedDeviceId = writable<string>('');
export const bootstrapStore = writable<BootstrapState | null>(null);
export const sessionToken = writable<string>('');
export const parentLocked = writable<boolean>(false);
export const toastMessage = writable<{ text: string; type: 'info' | 'success' | 'error' } | null>(null);

let toastTimeout: ReturnType<typeof setTimeout> | null = null;

export function showToast(text: string, type: 'info' | 'success' | 'error' = 'info') {
  toastMessage.set({ text, type });
  if (toastTimeout) clearTimeout(toastTimeout);
  toastTimeout = setTimeout(() => {
    toastMessage.set(null);
  }, 4000);
}

export function formatMinutes(totalMinutes: number): string {
  if (!totalMinutes || totalMinutes <= 0) return '0m';
  const h = Math.floor(totalMinutes / 60);
  const m = totalMinutes % 60;
  if (h === 0) return `${m}m`;
  if (m === 0) return `${h}h`;
  return `${h}h ${m}m`;
}

export function formatTimestamp(iso: string): string {
  if (!iso) return 'Never';
  try {
    const d = new Date(iso);
    if (isNaN(d.getTime())) return iso;
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) + ' · ' + d.toLocaleDateString();
  } catch {
    return iso;
  }
}

export function validateDomainInput(raw: string): string | null {
  const cleaned = raw
    .trim()
    .toLowerCase()
    .replace(/^https?:\/\//, '')
    .split('/')[0];
  if (!cleaned) return null;
  const re = /^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}$/;
  return re.test(cleaned) ? cleaned : null;
}
