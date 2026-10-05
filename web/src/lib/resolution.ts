import type { Resolution } from '../types/board'

/** Human wording of each resolution, used by the card badge and the drawer. */
export const RESOLUTION_LABELS: Record<Resolution, string> = {
  done: 'done',
  obsolete: 'obsolete',
  duplicate: 'duplicate',
  superseded: 'superseded',
}

/** True for a resolution that marks the spec as dropped (not delivered work):
 *  everything but `done`. Absent = legacy close, read as done. */
export function isDroppedResolution(resolution: Resolution | undefined): resolution is Exclude<Resolution, 'done'> {
  return resolution !== undefined && resolution !== 'done'
}
