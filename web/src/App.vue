<template>
  <div class="app-root" :class="{ 'dark': isDark }">
    <!-- Top Navbar -->
    <Navbar 
      :title="serverInfo?.title || 'awdex'"
      :breadcrumbs="breadcrumbs"
      v-model:viewMode="viewMode"
      :is-dark="isDark"
      :sidebar-open="sidebarOpen"
      :read-only="serverInfo?.read_only"
      @navigate="navigateTo"
      @toggle-sidebar="toggleSidebar"
      @open-search="showSearch = true"
      @open-upload="showUpload = true"
      @refresh="loadDirectory(currentPath)"
      @toggle-theme="toggleTheme"
      @open-shares="openShareModal(null)"
      @open-info="showInfo = true"
    />

    <!-- Main Workspace Area -->
    <div 
      class="app-workspace"
      @dragover.prevent="onWindowDragOver"
      @dragleave.prevent="onWindowDragLeave"
      @drop.prevent="onWindowDrop"
    >
      <!-- Mobile Drawer Backdrop -->
      <div 
        v-if="sidebarOpen" 
        class="sidebar-backdrop" 
        @click="sidebarOpen = false"
      ></div>

      <!-- Sidebar Folder Tree -->
      <SidebarTree 
        :is-open="sidebarOpen"
        :current-path="currentPath"
        @navigate="onSidebarNavigate"
        @toggle="toggleSidebar"
      />

      <!-- Content Area -->
      <main class="main-content" :class="{ 'has-audio-dock': !!currentTrack }">
        <div v-if="loading" class="content-loading">
          <div class="spinner"></div>
          <span>Loading directory...</span>
        </div>

        <FileList 
          v-else
          :items="items"
          :current-path="currentPath"
          :parent-path="parentPath"
          :view-mode="viewMode"
          :read-only="serverInfo?.read_only"
          :total-dirs="totalDirs"
          :total-files="totalFiles"
          :total-bytes="totalBytes"
          @navigate="navigateTo"
          @preview="openPreview"
          @play-audio="playAudio"
          @delete="handleDeleteItem"
          @select="selectedItem = $event"
          @share="openShareModal"
        />
      </main>

      <!-- Global Drag & Drop Overlay -->
      <div v-if="isWindowDragging && !serverInfo?.read_only" class="window-drop-overlay">
        <div class="drop-notice">
          <UploadCloud :size="64" />
          <h2>Drop files to upload to {{ currentPath }}</h2>
        </div>
      </div>
    </div>

    <!-- Docked Audio Player -->
    <AudioPlayer 
      v-if="currentTrack" 
      :track="currentTrack" 
      @close="currentTrack = null"
      @next="playNextAudio"
      @prev="playPrevAudio"
    />

    <!-- Preview Modal -->
    <PreviewModal 
      v-if="previewItem" 
      :item="previewItem" 
      :has-prev="hasPrevPreview"
      :has-next="hasNextPreview"
      @close="previewItem = null"
      @prev="navPreview(-1)"
      @next="navPreview(1)"
    />

    <!-- Upload Modal -->
    <UploadModal 
      v-if="showUpload" 
      :current-path="currentPath"
      @close="showUpload = false"
      @uploaded="loadDirectory(currentPath)"
    />

    <!-- Global Search Modal -->
    <SearchModal 
      v-if="showSearch" 
      @close="showSearch = false"
      @navigate="navigateTo"
      @select-item="openPreview"
    />

    <!-- Share Modal -->
    <ShareModal 
      v-if="showShareModal" 
      :item="shareTargetItem" 
      @close="showShareModal = false" 
    />

    <!-- Info Modal -->
    <InfoModal 
      v-if="showInfo" 
      :info="serverInfo" 
      @close="showInfo = false" 
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { UploadCloud } from '@lucide/vue'
import Navbar from './components/Navbar.vue'
import SidebarTree from './components/SidebarTree.vue'
import FileList from './components/FileList.vue'
import AudioPlayer from './components/AudioPlayer.vue'
import PreviewModal from './components/preview/PreviewModal.vue'
import UploadModal from './components/UploadModal.vue'
import SearchModal from './components/SearchModal.vue'
import InfoModal from './components/InfoModal.vue'
import ShareModal from './components/ShareModal.vue'
import { fetchBrowse, fetchInfo, deleteItem } from './utils/api.js'

// State
const currentPath = ref('/')
const parentPath = ref('/')
const breadcrumbs = ref([])
const items = ref([])
const totalDirs = ref(0)
const totalFiles = ref(0)
const totalBytes = ref(0)
const loading = ref(true)

const isMobileViewport = () => window.innerWidth <= 768

const viewMode = ref(localStorage.getItem('awdex-view-mode') || 'details')
const isDark = ref(localStorage.getItem('awdex-theme') !== 'light')
const sidebarOpen = ref(
  isMobileViewport() 
    ? false 
    : localStorage.getItem('awdex-sidebar') !== 'false'
)

watch(viewMode, (val) => {
  localStorage.setItem('awdex-view-mode', val)
})

function toggleSidebar() {
  sidebarOpen.value = !sidebarOpen.value
  if (!isMobileViewport()) {
    localStorage.setItem('awdex-sidebar', sidebarOpen.value ? 'true' : 'false')
  }
}

function onSidebarNavigate(path) {
  navigateTo(path)
  if (isMobileViewport()) {
    sidebarOpen.value = false
  }
}

const selectedItem = ref(null)
const previewItem = ref(null)
const currentTrack = ref(null)
const serverInfo = ref(null)

const showUpload = ref(false)
const showSearch = ref(false)
const showInfo = ref(false)
const showShareModal = ref(false)
const shareTargetItem = ref(null)

function openShareModal(item = null) {
  shareTargetItem.value = item
  showShareModal.value = true
}

const isWindowDragging = ref(false)
let dragCounter = 0

// Audio Playlist in current directory
const audioTracks = computed(() => {
  return items.value.filter(it => it.category === 'audio')
})

function playAudio(track) {
  currentTrack.value = track
}

function playNextAudio() {
  if (!currentTrack.value || audioTracks.value.length === 0) return
  const idx = audioTracks.value.findIndex(t => t.path === currentTrack.value.path)
  const nextIdx = (idx + 1) % audioTracks.value.length
  currentTrack.value = audioTracks.value[nextIdx]
}

function playPrevAudio() {
  if (!currentTrack.value || audioTracks.value.length === 0) return
  const idx = audioTracks.value.findIndex(t => t.path === currentTrack.value.path)
  const prevIdx = (idx - 1 + audioTracks.value.length) % audioTracks.value.length
  currentTrack.value = audioTracks.value[prevIdx]
}

// Preview Navigation
const previewableItems = computed(() => {
  return items.value.filter(it => !it.is_dir)
})

const hasPrevPreview = computed(() => {
  if (!previewItem.value) return false
  const idx = previewableItems.value.findIndex(i => i.path === previewItem.value.path)
  return idx > 0
})

const hasNextPreview = computed(() => {
  if (!previewItem.value) return false
  const idx = previewableItems.value.findIndex(i => i.path === previewItem.value.path)
  return idx !== -1 && idx < previewableItems.value.length - 1
})

function openPreview(item) {
  if (item.category === 'audio') {
    playAudio(item)
    return
  }
  previewItem.value = item
}

function navPreview(direction) {
  if (!previewItem.value) return
  const idx = previewableItems.value.findIndex(i => i.path === previewItem.value.path)
  if (idx !== -1 && previewableItems.value[idx + direction]) {
    previewItem.value = previewableItems.value[idx + direction]
  }
}

// Navigation & Directory loading
async function loadDirectory(path) {
  loading.value = true
  try {
    const data = await fetchBrowse(path)
    currentPath.value = data.path
    parentPath.value = data.parent_path
    breadcrumbs.value = data.breadcrumbs || []
    items.value = data.items || []
    totalDirs.value = data.total_dirs || 0
    totalFiles.value = data.total_files || 0
    totalBytes.value = data.total_bytes || 0
    selectedItem.value = null

    // Sync URL hash
    window.location.hash = encodeURI(data.path)
  } catch (err) {
    console.error('Failed to load path:', err)
  } finally {
    loading.value = false
  }
}

function navigateTo(path) {
  loadDirectory(path)
}

async function handleDeleteItem(item) {
  if (!confirm(`Are you sure you want to delete "${item.name}"?`)) return
  try {
    await deleteItem(item.path)
    loadDirectory(currentPath.value)
  } catch (err) {
    alert('Failed to delete: ' + err.message)
  }
}

function updateThemeColorMeta(dark) {
  const color = dark ? '#1e293b' : '#ffffff'
  const metaList = document.querySelectorAll('meta[name="theme-color"]')
  if (metaList.length > 0) {
    metaList.forEach(m => m.setAttribute('content', color))
  } else {
    const m = document.createElement('meta')
    m.name = 'theme-color'
    m.content = color
    document.head.appendChild(m)
  }
}

function applyTheme(dark) {
  isDark.value = dark
  const root = document.documentElement
  if (dark) {
    root.classList.add('dark')
    root.classList.remove('light')
    root.style.colorScheme = 'dark'
  } else {
    root.classList.remove('dark')
    root.classList.add('light')
    root.style.colorScheme = 'light'
  }
  updateThemeColorMeta(dark)
}

function toggleTheme() {
  const newDark = !isDark.value
  applyTheme(newDark)
  localStorage.setItem('awdex-theme', newDark ? 'dark' : 'light')
}

// Window Drag & Drop
function onWindowDragOver(e) {
  if (serverInfo.value?.read_only) return
  dragCounter++
  isWindowDragging.value = true
}

function onWindowDragLeave(e) {
  dragCounter--
  if (dragCounter <= 0) {
    isWindowDragging.value = false
    dragCounter = 0
  }
}

function onWindowDrop(e) {
  isWindowDragging.value = false
  dragCounter = 0
  if (serverInfo.value?.read_only) return
  if (e.dataTransfer?.files?.length > 0) {
    showUpload.value = true
  }
}

// Keyboard shortcuts
function handleGlobalKeydown(e) {
  // Ctrl+K / Cmd+K -> Search
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    showSearch.value = true
    return
  }

  // If typing in input, ignore file shortcuts
  if (['INPUT', 'TEXTAREA'].includes(document.activeElement?.tagName)) {
    return
  }

  // Space -> QuickLook preview
  if (e.code === 'Space') {
    if (selectedItem.value && !selectedItem.value.is_dir && !previewItem.value) {
      e.preventDefault()
      openPreview(selectedItem.value)
    }
  }

  // Backspace -> Parent directory
  if (e.key === 'Backspace' && !previewItem.value && !showSearch.value && !showUpload.value) {
    if (currentPath.value !== '/') {
      e.preventDefault()
      navigateTo(parentPath.value)
    }
  }
}

onMounted(async () => {
  applyTheme(isDark.value)
  window.addEventListener('keydown', handleGlobalKeydown)
  window.addEventListener('storage', (e) => {
    if (e.key === 'awdex-theme') {
      applyTheme(e.newValue !== 'light')
    }
  })

  // Listen to hash changes for browser back/forward buttons
  window.addEventListener('hashchange', () => {
    const hashPath = decodeURI(window.location.hash.slice(1)) || '/'
    if (hashPath !== currentPath.value) {
      loadDirectory(hashPath)
    }
  })

  // Load server info
  try {
    serverInfo.value = await fetchInfo()
  } catch (err) {
    console.error('Failed to load server info:', err)
  }

  // Initial path from hash or root
  const initialPath = decodeURI(window.location.hash.slice(1)) || '/'
  loadDirectory(initialPath)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleGlobalKeydown)
})
</script>

<style scoped>
.app-root {
  display: flex;
  flex-direction: column;
  height: 100dvh;
  width: 100%;
  overflow: hidden;
  background-color: var(--bg-app);
}

.app-workspace {
  flex: 1;
  display: flex;
  min-height: 0;
  height: 100%;
  max-height: 100%;
  overflow: hidden;
  position: relative;
}

.sidebar-backdrop {
  display: none;
}

@media (max-width: 768px) {
  .sidebar-backdrop {
    display: block;
    position: absolute;
    inset: 0;
    background-color: rgba(0, 0, 0, 0.55);
    backdrop-filter: blur(2px);
    z-index: 35;
  }
}

.main-content {
  flex: 1;
  min-inline-size: 0;
  min-width: 0;
  min-height: 0;
  height: 100%;
  max-height: 100%;
  overflow: hidden;
  position: relative;
  background-color: var(--bg-app);
}

.main-content.has-audio-dock {
  padding-bottom: calc(68px + env(safe-area-inset-bottom, 0px));
}

.content-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  gap: 12px;
  color: var(--text-muted);
  font-size: 14px;
}

.spinner {
  width: 32px;
  height: 32px;
  border: 3px solid var(--border-subtle);
  border-top-color: var(--primary);
  border-radius: 50%;
  animation: spin 0.75s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.window-drop-overlay {
  position: absolute;
  inset: 0;
  background-color: rgba(37, 99, 235, 0.25);
  backdrop-filter: blur(6px);
  z-index: 90;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 4px dashed var(--primary);
  pointer-events: none;
}

.drop-notice {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-strong);
  padding: 32px 48px;
  border-radius: var(--radius-xl);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  color: var(--primary);
  box-shadow: var(--shadow-xl);
}
</style>
