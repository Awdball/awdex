<template>
  <div class="file-list-container">
    <!-- Filter bar & directory stats -->
    <div class="list-toolbar">
      <div class="quick-filter-wrapper">
        <Filter :size="14" class="filter-icon" />
        <input 
          type="text" 
          v-model="filterQuery" 
          placeholder="Filter in folder... (Esc to clear)" 
          class="quick-filter-input"
          @keydown.esc="filterQuery = ''"
        />
        <span v-if="filterQuery" class="filter-count">
          {{ filteredItems.length }} / {{ items.length }}
        </span>
      </div>

      <div class="dir-summary">
        <span v-if="totalDirs > 0">{{ totalDirs }} {{ totalDirs === 1 ? 'folder' : 'folders' }}</span>
        <span v-if="totalDirs > 0 && totalFiles > 0" class="bullet">•</span>
        <span v-if="totalFiles > 0">{{ totalFiles }} {{ totalFiles === 1 ? 'file' : 'files' }}</span>
        <span v-if="totalBytes > 0" class="bullet">•</span>
        <span v-if="totalBytes > 0">{{ formatBytes(totalBytes) }}</span>
      </div>
    </div>

    <!-- Empty Directory State -->
    <div v-if="filteredItems.length === 0" class="empty-state">
      <FolderArchive :size="48" class="empty-icon" />
      <p class="empty-title">{{ filterQuery ? 'No matching items' : 'This folder is empty' }}</p>
      <p v-if="!readOnly && !filterQuery" class="empty-desc">Drop files here or click Upload to add contents</p>
    </div>

    <!-- 1. DETAILS / TABLE VIEW -->
    <div 
      v-else-if="viewMode === 'details'" 
      class="details-view" 
      @scroll.passive="onScroll"
      role="region" 
      aria-label="Directory contents" 
      tabindex="0"
    >
      <table class="file-table">
        <thead>
          <tr>
            <th class="th-name" @click="toggleSort('name')">
              Name
              <span v-if="sortField === 'name'" class="sort-indicator">{{ sortAsc ? '▲' : '▼' }}</span>
            </th>
            <th class="th-date" @click="toggleSort('date')">
              Modified
              <span v-if="sortField === 'date'" class="sort-indicator">{{ sortAsc ? '▲' : '▼' }}</span>
            </th>
            <th class="th-size" @click="toggleSort('size')">
              Size
              <span v-if="sortField === 'size'" class="sort-indicator">{{ sortAsc ? '▲' : '▼' }}</span>
            </th>
            <th class="th-type" @click="toggleSort('type')">
              Type
              <span v-if="sortField === 'type'" class="sort-indicator">{{ sortAsc ? '▲' : '▼' }}</span>
            </th>
            <th class="th-actions"></th>
          </tr>
        </thead>
        <tbody>
          <!-- Parent Directory Row if not in root -->
          <tr v-if="currentPath !== '/'" class="table-row parent-row" @click="$emit('navigate', parentPath)">
            <td class="td-name">
              <div class="td-name-inner">
                <div class="icon-thumb-wrapper">
                  <CornerLeftUp :size="16" class="item-icon parent-icon" />
                </div>
                <div class="name-text-stack">
                  <span class="file-name">.. (Parent Directory)</span>
                </div>
              </div>
            </td>
            <td class="td-date">-</td>
            <td class="td-size">-</td>
            <td class="td-type">-</td>
            <td class="td-actions"></td>
          </tr>

          <!-- Item Rows -->
          <tr 
            v-for="item in visibleItems" 
            :key="item.path" 
            class="table-row" 
            :class="{ 'is-selected': selectedItem?.path === item.path }"
            @click="handleItemClick(item)"
          >
            <td class="td-name">
              <div class="td-name-inner">
                <!-- Item Icon or mini thumbnail -->
                <div class="icon-thumb-wrapper">
                  <img 
                    v-if="item.has_thumb && !failedThumbs[item.path]" 
                    :src="item.thumb_url" 
                    class="mini-thumb" @error="failedThumbs[item.path] = true" 
                    loading="lazy" 
                    alt=""
                  />
                  <component 
                    v-else 
                    :is="getItemIcon(item)" 
                    :size="18" 
                    class="item-icon" 
                    :style="{ color: getItemColor(item) }" 
                  />
                </div>

                <div class="name-text-stack">
                  <span class="file-name" :title="item.name">{{ item.name }}</span>
                  <span class="mobile-meta">
                    <template v-if="item.is_dir">
                      Folder<template v-if="item.item_count"> • {{ item.item_count }} items</template>
                      <template v-if="item.mod_time"> • {{ formatDate(item.mod_time) }}</template>
                    </template>
                    <template v-else>
                      {{ formatBytes(item.size) }} • {{ item.extension ? item.extension.slice(1).toUpperCase() : item.category }} • {{ formatDate(item.mod_time) }}
                    </template>
                  </span>
                </div>
              </div>
            </td>
            <td class="td-date">{{ formatDate(item.mod_time) }}</td>
            <td class="td-size">{{ item.is_dir ? (item.item_count ? `${item.item_count} items` : '-') : formatBytes(item.size) }}</td>
            <td class="td-type">
              <span class="category-pill" :style="{ backgroundColor: getCategoryBadgeColor(item.category) + '22', color: getCategoryBadgeColor(item.category) }">
                {{ item.is_dir ? 'folder' : (item.extension ? item.extension.slice(1).toUpperCase() : item.category) }}
              </span>
            </td>
            <td class="td-actions" @click.stop>
              <div class="row-actions">
                <button 
                  class="action-icon-btn" 
                  title="Share Public Link" 
                  @click="$emit('share', item)"
                >
                  <Share2 :size="15" />
                </button>
                <a 
                  v-if="!item.is_dir" 
                  :href="'/raw' + encodePath(item.path) + '?download=1'" 
                  class="action-icon-btn" 
                  title="Download File" 
                  download
                >
                  <Download :size="15" />
                </a>
                <button 
                  v-if="!item.is_dir" 
                  class="action-icon-btn" 
                  title="Preview" 
                  @click="$emit('preview', item)"
                >
                  <Eye :size="15" />
                </button>
                <button 
                  v-if="!readOnly" 
                  class="action-icon-btn is-danger" 
                  title="Delete" 
                  @click="$emit('delete', item)"
                >
                  <Trash2 :size="15" />
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 2. GRID / THUMBNAIL VIEW -->
    <div v-else-if="viewMode === 'grid'" class="grid-view" @scroll.passive="onScroll">
      <!-- Parent directory card -->
      <div v-if="currentPath !== '/'" class="grid-card parent-card" @click="$emit('navigate', parentPath)">
        <div class="card-thumb-area parent-thumb-area">
          <CornerLeftUp :size="32" class="parent-card-icon" />
        </div>
        <div class="card-footer">
          <div class="card-info">
            <span class="card-title">.. (Parent Directory)</span>
          </div>
        </div>
      </div>

      <!-- Item cards -->
      <div 
        v-for="item in visibleItems" 
        :key="item.path" 
        class="grid-card" 
        :class="{ 'is-selected': selectedItem?.path === item.path }"
        @click="handleItemClick(item)"
        @mouseenter="hoverItem = item"
        @mouseleave="hoverItem = null"
      >
        <div class="card-thumb-area">
          <!-- Thumbnail preview or Animated video scrub on hover -->
          <img 
            v-if="item.has_thumb && !failedThumbs[item.path]" 
            :src="hoverItem === item && item.scrub_url ? item.scrub_url : (item.thumb_url || '/raw' + encodePath(item.path))" 
            class="card-thumb-img" @error="failedThumbs[item.path] = true" 
            loading="lazy" 
            alt=""
          />
          <div v-else class="card-icon-placeholder" :style="{ color: getItemColor(item) }">
            <component :is="getItemIcon(item)" :size="48" />
          </div>

          <!-- Folder overlay badge when folder has a thumbnail -->
          <div v-if="item.is_dir && item.has_thumb && !failedThumbs[item.path]" class="card-overlay-badge folder-badge">
            <Folder :size="13" />
          </div>

          <!-- Video play overlay indicator -->
          <div v-if="item.category === 'video'" class="card-overlay-badge video-badge">
            <Play :size="14" />
          </div>

          <!-- Audio overlay indicator -->
          <div v-if="item.category === 'audio'" class="card-overlay-badge audio-badge">
            <Music :size="14" />
          </div>

          <!-- Size tag badge -->
          <span v-if="!item.is_dir" class="card-size-tag">{{ formatBytes(item.size) }}</span>
        </div>

        <div class="card-footer">
          <div class="card-info">
            <span class="card-title" :title="item.name">{{ item.name }}</span>
            <span class="card-subtitle">{{ item.is_dir ? `${item.item_count || 0} items` : `${formatBytes(item.size)} • ${formatDate(item.mod_time)}` }}</span>
          </div>

          <div class="card-actions" :class="{ 'is-dir': item.is_dir }" @click.stop>
            <button 
              class="card-btn" 
              title="Share Public Link" 
              @click="$emit('share', item)"
            >
              <Share2 :size="15" />
            </button>
            <template v-if="!item.is_dir">
              <a 
                :href="'/raw' + encodePath(item.path) + '?download=1'" 
                class="card-btn" 
                title="Download" 
                download
              >
                <Download :size="15" />
              </a>
              <button 
                class="card-btn" 
                title="Preview" 
                @click="$emit('preview', item)"
              >
                <Eye :size="15" />
              </button>
            </template>
          </div>
        </div>
      </div>
    </div>

    <!-- 3. COMPACT ICONS VIEW -->
    <div v-else class="icons-view" @scroll.passive="onScroll">
      <div v-if="currentPath !== '/'" class="icon-item parent-item" @click="$emit('navigate', parentPath)">
        <CornerLeftUp :size="24" class="parent-icon" />
        <span class="icon-label">..</span>
      </div>

      <div 
        v-for="item in visibleItems" 
        :key="item.path" 
        class="icon-item" 
        :class="{ 'is-selected': selectedItem?.path === item.path }"
        @click="handleItemClick(item)"
      >
        <component :is="getItemIcon(item)" :size="28" class="icon-symbol" :style="{ color: getItemColor(item) }" />
        <span class="icon-label" :title="item.name">{{ item.name }}</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { 
  Filter, CornerLeftUp, Download, Eye, Trash2, Play, Music, Share2,
  Folder, FolderArchive, FileText, FileCode, FileSpreadsheet, 
  FileImage, FileVideo, FileAudio, File, Archive
} from '@lucide/vue'
import { formatBytes, formatDate, getCategoryBadgeColor, encodePath } from '../utils/formatters.js'

const props = defineProps({
  items: {
    type: Array,
    default: () => []
  },
  currentPath: String,
  parentPath: String,
  viewMode: {
    type: String,
    default: 'details'
  },
  readOnly: Boolean,
  totalDirs: Number,
  totalFiles: Number,
  totalBytes: Number
})

const emit = defineEmits(['navigate', 'preview', 'play-audio', 'delete', 'select', 'share'])

const filterQuery = ref('')
const sortField = ref('name')
const sortAsc = ref(true)
const selectedItem = ref(null)
const hoverItem = ref(null)
const failedThumbs = ref({})

function toggleSort(field) {
  if (sortField.value === field) {
    sortAsc.value = !sortAsc.value
  } else {
    sortField.value = field
    sortAsc.value = true
  }
}

const filteredItems = computed(() => {
  let list = props.items
  if (filterQuery.value.trim()) {
    const q = filterQuery.value.toLowerCase()
    list = list.filter(it => it.name.toLowerCase().includes(q))
  }

  return [...list].sort((a, b) => {
    // Directories always come first
    if (a.is_dir !== b.is_dir) {
      return a.is_dir ? -1 : 1
    }

    let cmp = 0
    if (sortField.value === 'name') {
      const an = a.name.toLowerCase()
      const bn = b.name.toLowerCase()
      cmp = an < bn ? -1 : (an > bn ? 1 : 0)
    } else if (sortField.value === 'size') {
      cmp = (a.size || 0) - (b.size || 0)
    } else if (sortField.value === 'date') {
      cmp = new Date(a.mod_time) - new Date(b.mod_time)
    } else if (sortField.value === 'type') {
      const at = a.category || ''
      const bt = b.category || ''
      cmp = at < bt ? -1 : (at > bt ? 1 : 0)
    }

    return sortAsc.value ? cmp : -cmp
  })
})

const itemsToShow = ref(100)

watch(() => [props.currentPath, filterQuery.value], () => {
  itemsToShow.value = 100
})

const visibleItems = computed(() => {
  return filteredItems.value.slice(0, itemsToShow.value)
})

function onScroll(e) {
  const el = e.target
  if (el.scrollTop + el.clientHeight >= el.scrollHeight - 400) {
    if (itemsToShow.value < filteredItems.value.length) {
      itemsToShow.value = Math.min(itemsToShow.value + 100, filteredItems.value.length)
    }
  }
}

function selectItem(item) {
  selectedItem.value = item
  emit('select', item)
}

function openItem(item) {
  if (item.is_dir) {
    emit('navigate', item.path)
  } else if (item.category === 'audio') {
    emit('play-audio', item)
  } else {
    emit('preview', item)
  }
}

function handleItemClick(item) {
  selectItem(item)
  openItem(item)
}

function getItemIcon(item) {
  if (item.is_dir) return Folder
  switch (item.category) {
    case 'image': return FileImage
    case 'video': return FileVideo
    case 'audio': return FileAudio
    case 'pdf': return FileText
    case 'code': return FileCode
    case 'text': return FileText
    case 'archive': return Archive
    default: return File
  }
}

function getItemColor(item) {
  if (item.is_dir) return '#f59e0b'
  return getCategoryBadgeColor(item.category)
}
</script>

<style scoped>
.file-list-container {
  padding: 16px 20px;
  width: 100%;
  max-inline-size: 100%;
  height: 100%;
  max-height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  container-type: inline-size;
  container-name: filelist;
}

.list-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
  gap: 12px;
  flex-wrap: wrap;
  flex-shrink: 0;
  background-color: var(--bg-app);
  padding: 4px 0;
}

.quick-filter-wrapper {
  display: flex;
  align-items: center;
  background-color: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  padding: 5px 10px;
  gap: 8px;
  width: 280px;
  max-width: 100%;
  flex: 1 1 200px;
}

.quick-filter-wrapper:focus-within {
  border-color: var(--primary);
}

.filter-icon {
  color: var(--text-subtle);
  flex-shrink: 0;
}

.quick-filter-input {
  background: transparent;
  border: none;
  color: var(--text-main);
  font-size: 13px;
  flex: 1;
  min-width: 0;
}

.filter-count {
  font-size: 11px;
  color: var(--text-subtle);
  font-family: var(--font-mono);
  flex-shrink: 0;
}

.dir-summary {
  font-size: 12px;
  color: var(--text-muted);
  display: flex;
  align-items: center;
  gap: 6px;
}

.bullet {
  color: var(--text-subtle);
}

/* Empty state */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 80px 20px;
  color: var(--text-muted);
  text-align: center;
}

.empty-icon {
  color: var(--text-subtle);
  margin-bottom: 12px;
  opacity: 0.6;
}

.empty-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-main);
}

.empty-desc {
  font-size: 13px;
  color: var(--text-subtle);
  margin-top: 4px;
}

/* 1. Details Table */
.details-view {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-sm);
  outline: none;
  width: 100%;
  max-inline-size: 100%;
  flex: 0 1 auto;
  min-height: 0;
  max-height: 100%;
  overflow-y: auto;
  overflow-x: hidden;
  overscroll-behavior: contain;
}

.file-table {
  width: 100%;
  max-inline-size: 100%;
  table-layout: fixed;
  border-collapse: separate;
  border-spacing: 0;
  font-size: 13px;
  text-align: left;
}

thead {
  position: sticky;
  inset-block-start: 0;
  z-index: 5;
}

th {
  background-color: var(--bg-surface-hover);
  color: var(--text-muted);
  font-weight: 600;
  padding: 9px 12px;
  border-bottom: 1px solid var(--border-subtle);
  cursor: pointer;
  user-select: none;
}

th:hover {
  color: var(--text-main);
}

.th-name { width: 52%; }
.th-date { width: 18%; }
.th-size { width: 12%; }
.th-type { width: 10%; }
.th-actions { width: 8%; text-align: right; }

.table-row {
  cursor: pointer;
  transition: background-color 0.1s;
}

.table-row td {
  border-bottom: 1px solid var(--border-subtle);
}

.table-row:last-child td {
  border-bottom: none;
}

.table-row:hover {
  background-color: var(--bg-surface-hover);
}

.table-row.is-selected {
  background-color: var(--primary-subtle);
}

td {
  padding: 9px 12px;
  color: var(--text-muted);
  vertical-align: middle;
}

.td-name {
  color: var(--text-main);
  font-weight: 500;
  max-inline-size: 0;
  overflow: hidden;
}

.td-name-inner {
  display: flex;
  align-items: center;
  gap: 10px;
  min-inline-size: 0;
  min-width: 0;
  width: 100%;
}

.name-text-stack {
  display: flex;
  flex-direction: column;
  min-inline-size: 0;
  min-width: 0;
  flex: 1;
  overflow: hidden;
}

.file-name {
  display: block;
  max-inline-size: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mobile-meta {
  display: none;
  font-size: 11px;
  font-weight: 400;
  color: var(--text-subtle);
  margin-top: 2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.icon-thumb-wrapper {
  width: 26px;
  height: 26px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.mini-thumb {
  width: 26px;
  height: 26px;
  border-radius: 4px;
  object-fit: cover;
}

.category-pill {
  font-size: 10px;
  font-weight: 700;
  padding: 2px 6px;
  border-radius: 4px;
  text-transform: uppercase;
}

.row-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 4px;
  opacity: 0;
  transition: opacity 0.15s;
}

.table-row:hover .row-actions {
  opacity: 1;
}

.action-icon-btn {
  width: 28px;
  height: 28px;
  border-radius: var(--radius-sm);
  color: var(--text-muted);
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.action-icon-btn:hover {
  background-color: var(--bg-surface-active);
  color: var(--text-main);
}

.action-icon-btn.is-danger:hover {
  color: #ef4444;
}

/* 2. Grid Cards */
.grid-view {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 16px;
  width: 100%;
  max-inline-size: 100%;
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  overscroll-behavior: contain;
  align-content: start;
  padding-bottom: 8px;
}

.grid-card {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  overflow: hidden;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  transition: all 0.18s ease;
  box-shadow: var(--shadow-sm);
  min-width: 0;
}

.grid-card:hover {
  transform: translateY(-2px);
  border-color: var(--border-strong);
  box-shadow: var(--shadow-md);
}

.grid-card.is-selected {
  border-color: var(--primary);
  box-shadow: 0 0 0 2px var(--primary-subtle);
}

.card-thumb-area {
  position: relative;
  width: 100%;
  aspect-ratio: 16 / 10;
  min-height: 110px;
  background-color: var(--bg-app);
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  flex-shrink: 0;
}

.card-thumb-img {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
  transition: transform 0.25s ease;
}

.card-icon-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
}

.parent-thumb-area {
  background-color: var(--bg-surface-hover);
}

.parent-card-icon {
  color: var(--text-muted);
}

.grid-card:hover .card-thumb-img {
  transform: scale(1.03);
}

.card-overlay-badge {
  position: absolute;
  top: 8px;
  right: 8px;
  background: rgba(0, 0, 0, 0.65);
  backdrop-filter: blur(4px);
  color: white;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 2;
}

.card-size-tag {
  position: absolute;
  bottom: 6px;
  right: 6px;
  background: rgba(0, 0, 0, 0.65);
  backdrop-filter: blur(4px);
  color: white;
  font-size: 10px;
  font-family: var(--font-mono);
  padding: 1px 5px;
  border-radius: 3px;
  z-index: 2;
}

.card-footer {
  padding: 8px 10px 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1 0 auto;
  min-height: 72px;
  background-color: var(--bg-surface);
}

.card-info {
  width: 100%;
  min-width: 0;
  overflow: hidden;
}

.card-title {
  display: block;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-main);
  white-space: normal;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  line-clamp: 2;
  overflow: hidden;
  line-height: 1.35;
  word-break: break-word;
}

.card-subtitle {
  display: block;
  font-size: 11px;
  color: var(--text-subtle);
  margin-top: 3px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.card-actions {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 4px;
  width: 100%;
  padding-top: 6px;
  border-top: 1px solid var(--border-subtle);
  margin-top: auto;
}

.card-actions.is-dir {
  grid-template-columns: 1fr;
}

.card-btn {
  width: 100%;
  height: 28px;
  border-radius: var(--radius-sm);
  color: var(--text-muted);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background-color: var(--bg-app);
  transition: all 0.15s ease;
}

.card-btn:hover {
  background-color: var(--bg-surface-hover);
  color: var(--text-main);
}

/* 3. Compact Icons View */
.icons-view {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(110px, 1fr));
  gap: 12px;
  width: 100%;
  max-inline-size: 100%;
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  overscroll-behavior: contain;
  align-content: start;
  padding-bottom: 8px;
}

.icon-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 12px 8px;
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: background-color 0.12s;
  text-align: center;
  min-width: 0;
}

.icon-item:hover {
  background-color: var(--bg-surface-hover);
}

.icon-item.is-selected {
  background-color: var(--primary-subtle);
  color: var(--primary);
}

.icon-symbol {
  margin-bottom: 6px;
}

.icon-label {
  font-size: 12px;
  color: var(--text-main);
  max-width: 100%;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* Touch devices always show row/card action buttons */
@media (hover: none) {
  .row-actions,
  .card-actions {
    opacity: 1;
  }
}

/* Container Queries for Narrow / Mobile Viewports */
@container filelist (width < 640px) {
  .th-date,
  .td-date,
  .th-size,
  .td-size,
  .th-type,
  .td-type {
    display: none;
  }

  .th-name,
  .td-name {
    width: calc(100% - 92px);
  }

  .th-actions,
  .td-actions {
    width: 92px;
  }

  .file-name {
    white-space: normal;
    overflow-wrap: anywhere;
    word-break: break-word;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    line-clamp: 2;
    overflow: clip;
    line-height: 1.35;
  }

  .mobile-meta {
    display: block;
  }

  .row-actions,
  .card-actions {
    opacity: 1;
  }

  td {
    padding: 10px 10px;
  }

  .grid-view {
    grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));
    gap: 10px;
  }

  .card-thumb-area {
    min-height: 80px;
  }
}

@media (max-width: 640px) {
  .grid-view {
    grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));
    gap: 10px;
  }

  .card-thumb-area {
    min-height: 80px;
  }
}

@media (max-width: 768px) {
  .file-list-container {
    padding: 10px 12px;
    padding-bottom: calc(10px + env(safe-area-inset-bottom, 0px));
  }
}
</style>
