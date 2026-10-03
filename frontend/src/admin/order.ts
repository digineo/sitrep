import type { PanelInfo } from "../shared/payload"

// Panels move only within the group of their type. The full order keeps
// each group in the positions the group had.

function withGroup(
  panels: PanelInfo[],
  type: PanelInfo["type"],
  group: string[],
): string[] {
  let k = 0
  return panels.map(p => p.type === type ? group[k++]! : p.id)
}

/** moveBy returns the panel IDs with id moved by `by` places within its group. */
export function moveBy(panels: PanelInfo[], id: string, by: number): string[] {
  const type = panels.find(p => p.id === id)!.type
  const group = panels.filter(p => p.type === type).map(p => p.id)
  const i = group.indexOf(id)
  const j = Math.min(group.length - 1, Math.max(0, i + by))
  group.splice(j, 0, ...group.splice(i, 1))
  return withGroup(panels, type, group)
}

/**
 * moveTo returns the panel IDs with id moved to target's place, if both
 * share a group.
 */
export function moveTo(panels: PanelInfo[], id: string, target: string): string[] {
  const type = panels.find(p => p.id === id)!.type
  const group = panels.filter(p => p.type === type).map(p => p.id)
  const j = group.indexOf(target)
  if (j >= 0) {
    group.splice(j, 0, ...group.splice(group.indexOf(id), 1))
  }
  return withGroup(panels, type, group)
}

/** ordered returns the panels in the order of ids. */
export function ordered(panels: PanelInfo[], ids: string[]): PanelInfo[] {
  return ids.flatMap(id => panels.find(p => p.id === id) ?? [])
}
