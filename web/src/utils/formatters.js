export function formatBytes(bytes) {
  if (bytes === 0 || bytes === undefined || bytes === null) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

export function formatDate(dateStr) {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return '-'
  
  const now = new Date()
  const isToday = d.toDateString() === now.toDateString()
  
  if (isToday) {
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }
  
  return d.toLocaleDateString([], { 
    year: d.getFullYear() !== now.getFullYear() ? 'numeric' : undefined,
    month: 'short', 
    day: 'numeric' 
  }) + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

export function getCategoryBadgeColor(category) {
  switch (category) {
    case 'video': return '#ef4444' // Red
    case 'audio': return '#8b5cf6' // Purple
    case 'image': return '#10b981' // Green
    case 'document': return '#f59e0b' // Amber
    case 'code': return '#06b6d4' // Cyan
    case 'archive': return '#ec4899' // Pink
    case 'text': return '#64748b' // Slate
    default: return '#94a3b8'
  }
}
