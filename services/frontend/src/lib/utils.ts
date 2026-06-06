import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";
import type { Event } from "./types.js";

export function cn(...inputs: ClassValue[]) {
	return twMerge(clsx(inputs));
}

export function fmtTime(iso: string): string {
  return new Date(iso).toLocaleTimeString('uk-UA', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

export function fmtDate(iso: string): string {
  return new Date(iso).toLocaleDateString('uk-UA', { day: 'numeric', month: 'short', year: 'numeric' });
}

export function fmtDateTime(iso: string): string {
  return `${fmtDate(iso)} ${fmtTime(iso)}`;
}

export function timeAgo(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  if (diff < 60_000) return 'щойно';
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)} хв тому`;
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)} год тому`;
  return fmtDate(iso);
}

export function eventSummary(events: Event[]): string {
  if (!events.length) return '—';
  const parts = events.flatMap(ev =>
    Object.entries(ev.data).slice(0, 2).map(([, v]) => String(v))
  ).slice(0, 3);
  return parts.join(' · ') || '—';
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type WithoutChild<T> = T extends { child?: any } ? Omit<T, "child"> : T;
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type WithoutChildren<T> = T extends { children?: any } ? Omit<T, "children"> : T;
export type WithoutChildrenOrChild<T> = WithoutChildren<WithoutChild<T>>;
export type WithElementRef<T, U extends HTMLElement = HTMLElement> = T & { ref?: U | null };
