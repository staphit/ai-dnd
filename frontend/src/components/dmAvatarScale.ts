/** Shared DM portrait scale limits (no GLB / three.js imports — safe for tests). */

/** Scene units for Mixamo ~1.7u figure (fits head-to-toe at default camera). */
export const DM_AVATAR_SCALE_DEFAULT = 0.72;
/** Noticeably smaller / larger than default so the slider feels effective. */
export const DM_AVATAR_SCALE_MIN = 0.4;
export const DM_AVATAR_SCALE_MAX = 1.2;

export const DM_SCALE_STORAGE_KEY = 'dnd-duet-dm-avatar-scale';

export function clampDmAvatarScale(value: number): number {
  if (!Number.isFinite(value)) return DM_AVATAR_SCALE_DEFAULT;
  return Math.min(DM_AVATAR_SCALE_MAX, Math.max(DM_AVATAR_SCALE_MIN, value));
}

export function loadStoredDmScale(): number {
  try {
    const raw = localStorage.getItem(DM_SCALE_STORAGE_KEY);
    if (raw == null) return DM_AVATAR_SCALE_DEFAULT;
    return clampDmAvatarScale(Number(raw));
  } catch {
    return DM_AVATAR_SCALE_DEFAULT;
  }
}

export function saveStoredDmScale(value: number): void {
  try {
    localStorage.setItem(DM_SCALE_STORAGE_KEY, String(clampDmAvatarScale(value)));
  } catch {
    /* ignore quota / private mode */
  }
}
