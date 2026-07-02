import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'
export function cn(...inputs: ClassValue[]) { return twMerge(clsx(inputs)) }
export const sevColor = (s: string) => ({
  critical: 'text-sev-critical', high: 'text-sev-high', medium: 'text-sev-medium', low: 'text-sev-low',
} as Record<string,string>)[s] ?? 'text-slate-400'
