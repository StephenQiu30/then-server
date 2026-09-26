export const scenes = [
  { id: 'daily', label: '日常出行', description: '轻松自在' },
  { id: 'commute', label: '通勤上班', description: '整洁得体' },
  { id: 'friends', label: '与朋友见面', description: '自在相聚' },
  { id: 'date', label: '约会聚会', description: '略带心意' },
  { id: 'travel', label: '旅行出游', description: '便于走动' },
  { id: 'relax', label: '居家放松', description: '舒适为先' },
] as const

export type SceneId = (typeof scenes)[number]['id']

export function isSceneId(value: string): value is SceneId {
  return scenes.some((scene) => scene.id === value)
}
