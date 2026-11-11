<template>
  <div 
    class="app-root" 
    :class="{ 'dark': isDark }"
    :data-browse-state="browseState"
    :data-active-modal="activeModal"
    :data-drag-state="dragState"
    :data-current-path="currentPath"
  >
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
      @open-search="openModal(MODAL_STATES.SEARCH)"
      @open-upload="openUploadModal([])"
      @refresh="loadDirectory(currentPath)"
      @toggle-theme="toggleTheme"
      @open-shares="openShareModal(null)"
      @open-info="openInfoModal"
    />

    <!-- Main Workspace Area -->
    <div 
      class="app-workspace"
      :data-state="browseState"
      @dragenter.prevent="onWindowDragEnter"
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
      <main class="main-content" :class="{ 'has-audio-dock': !!currentTrack }" :data-state="browseState">
        <div v-if="browseState === BROWSE_STATES.LOADING || browseState === BROWSE_STATES.IDLE" class="content-loading">
          <div class="spinner"></div>
          <span>Loading directory...</span>
        </div>

        <div v-else-if="browseState === BROWSE_STATES.ERROR" class="content-error" role="alert">
          <AlertCircle :size="44" class="error-icon" />
          <h3 class="error-title">Unable to load directory</h3>
          <p class="error-desc">{{ browseError || 'The requested path could not be read.' }}</p>
          <div class="error-actions">
            <button class="error-btn primary" @click="loadDirectory(currentPath)">
              <RefreshCw :size="15" />
              <span>Retry</span>
            </button>
            <button v-if="currentPath !== '/'" class="error-btn" @click="navigateTo('/')">
              <span>Back to Root (~)</span>
            </button>
          </div>
        </div>

        <FileList 
          v-else
          :items="items"
          :current-path="currentPath"
          :parent-path="parentPath"
          :view-mode="viewMode"
          :browse-state="browseState"
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
      <div v-if="dragState === DRAG_STATES.DRAGGING && !serverInfo?.read_only" class="window-drop-overlay">
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
      @close="closeAudioPlayer"
      @next="playNextAudio"
      @prev="playPrevAudio"
    />

    <!-- Preview Modal -->
    <PreviewModal 
      v-if="activeModal === MODAL_STATES.PREVIEW && previewItem" 
      :item="previewItem" 
      :has-prev="hasPrevPreview"
      :has-next="hasNextPreview"
      @close="closeModal"
      @prev="navPreview(-1)"
      @next="navPreview(1)"
    />

    <!-- Upload Modal -->
    <UploadModal 
      v-if="activeModal === MODAL_STATES.UPLOAD" 
      :current-path="currentPath"
      :initial-files="pendingDroppedFiles"
      @close="closeModal"
      @uploaded="onFilesUploaded"
    />

    <!-- Global Search Modal -->
    <SearchModal 
      v-if="activeModal === MODAL_STATES.SEARCH" 
      @close="closeModal"
      @navigate="navigateTo"
      @select-item="openPreview"
    />

    <!-- Share Modal -->
    <ShareModal 
      v-if="activeModal === MODAL_STATES.SHARE" 
      :item="shareTargetItem" 
      @close="closeModal" 
    />

    <!-- Info Modal -->
    <InfoModal 
      v-if="activeModal === MODAL_STATES.INFO" 
      :info="serverInfo" 
      @close="closeModal" 
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { UploadCloud, AlertCircle, RefreshCw } from '@lucide/vue'
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
import {
  BROWSE_STATES,
  MODAL_STATES,
  DRAG_STATES,
  AUDIO_STATES,
  syncAgentState
} from './utils/stateMachines.js'

// Directory Browse State Machine
const currentPath = ref('/')
const parentPath = ref('/')
const breadcrumbs = ref([])
const items = ref([])
const totalDirs = ref(0)
const totalFiles = ref(0)
const totalBytes = ref(0)
const browseState = ref(BROWSE_STATES.IDLE)
const browseError = ref(null)
let browseRequestId = 0
let browseAbortController = null

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

// Coordinated Top-Level Modal State Machine
const activeModal = ref(MODAL_STATES.NONE)
const selectedItem = ref(null)
const previewItem = ref(null)
const shareTargetItem = ref(null)
const pendingDroppedFiles = ref([])
const serverInfo = ref(null)

function openModal(modalType) {
  activeModal.value = modalType
  syncAgentState({ activeModal: modalType })
}

function closeModal() {
  if (activeModal.value === MODAL_STATES.PREVIEW) {
    previewItem.value = null
  } else if (activeModal.value === MODAL_STATES.UPLOAD) {
    pendingDroppedFiles.value = []
  } else if (activeModal.value === MODAL_STATES.SHARE) {
    shareTargetItem.value = null
  }
  activeModal.value = MODAL_STATES.NONE
  syncAgentState({ activeModal: MODAL_STATES.NONE })
}

function openUploadModal(files = []) {
  pendingDroppedFiles.value = Array.isArray(files) ? files : []
  openModal(MODAL_STATES.UPLOAD)
}

function openShareModal(item = null) {
  shareTargetItem.value = item
  openModal(MODAL_STATES.SHARE)
}

async function openInfoModal() {
  openModal(MODAL_STATES.INFO)
  await refreshServerInfo()
}

function onFilesUploaded() {
  pendingDroppedFiles.value = []
  loadDirectory(currentPath.value)
  refreshServerInfo()
}

// Window Drag & Drop State Machine
const dragState = ref(DRAG_STATES.IDLE)
let dragCounter = 0

function resetDragState() {
  dragCounter = 0
  if (dragState.value !== DRAG_STATES.IDLE) {
    dragState.value = DRAG_STATES.IDLE
    syncAgentState({ dragState: DRAG_STATES.IDLE })
  }
}

function onWindowDragEnter(e) {
  if (serverInfo.value?.read_only) return
  const types = e.dataTransfer?.types
  if (types && !Array.from(types).includes('Files')) return
  dragCounter++
  if (dragState.value !== DRAG_STATES.DRAGGING) {
    dragState.value = DRAG_STATES.DRAGGING
    syncAgentState({ dragState: DRAG_STATES.DRAGGING })
  }
}

function onWindowDragOver(e) {
  if (serverInfo.value?.read_only) return
  if (e.dataTransfer) {
    e.dataTransfer.dropEffect = 'copy'
  }
}

function onWindowDragLeave() {
  dragCounter--
  if (dragCounter <= 0) {
    resetDragState()
  }
}

function onWindowDrop(e) {
  resetDragState()
  if (serverInfo.value?.read_only) return
  const dropped = e.dataTransfer?.files ? Array.from(e.dataTransfer.files) : []
  if (dropped.length > 0) {
    openUploadModal(dropped)
  }
}

// Persistent Audio Playlist & Player State Machine
const currentTrack = ref(null)
const audioPlaylist = ref([])

const directoryAudioTracks = computed(() => {
  return items.value.filter(it => it.category === 'audio')
})

function playAudio(track) {
  const dirTracks = directoryAudioTracks.value
  if (dirTracks.length > 0 && dirTracks.some(t => t.path === track.path)) {
    audioPlaylist.value = [...dirTracks]
  } else if (!audioPlaylist.value.some(t => t.path === track.path)) {
    audioPlaylist.value = [track]
  }
  currentTrack.value = track
}

function closeAudioPlayer() {
  currentTrack.value = null
  audioPlaylist.value = []
  syncAgentState({ audioState: AUDIO_STATES.IDLE })
}

function playNextAudio(opts = {}) {
  const list = audioPlaylist.value.length > 0 ? audioPlaylist.value : directoryAudioTracks.value
  if (!currentTrack.value || list.length === 0) return
  const idx = list.findIndex(t => t.path === currentTrack.value.path)
  if (opts?.shuffle && list.length > 1) {
    let randIdx = idx
    while (randIdx === idx) {
      randIdx = Math.floor(Math.random() * list.length)
    }
    currentTrack.value = list[randIdx]
    return
  }
  const nextIdx = (idx + 1) % list.length
  currentTrack.value = list[nextIdx]
}

function playPrevAudio() {
  const list = audioPlaylist.value.length > 0 ? audioPlaylist.value : directoryAudioTracks.value
  if (!currentTrack.value || list.length === 0) return
  const idx = list.findIndex(t => t.path === currentTrack.value.path)
  const prevIdx = (idx - 1 + list.length) % list.length
  currentTrack.value = list[prevIdx]
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
  openModal(MODAL_STATES.PREVIEW)
}

function navPreview(direction) {
  if (!previewItem.value) return
  const idx = previewableItems.value.findIndex(i => i.path === previewItem.value.path)
  if (idx !== -1 && previewableItems.value[idx + direction]) {
    previewItem.value = previewableItems.value[idx + direction]
  }
}

// Navigation & Directory loading state machine with race cancellation
async function loadDirectory(path) {
  const reqId = ++browseRequestId
  if (browseAbortController) {
    browseAbortController.abort()
  }
  browseAbortController = new AbortController()

  browseState.value = BROWSE_STATES.LOADING
  browseError.value = null
  syncAgentState({ browseState: BROWSE_STATES.LOADING })

  try {
    const data = await fetchBrowse(path, { signal: browseAbortController.signal })
    if (reqId !== browseRequestId) return

    currentPath.value = data.path
    parentPath.value = data.parent_path
    breadcrumbs.value = data.breadcrumbs || []
    items.value = data.items || []
    totalDirs.value = data.total_dirs || 0
    totalFiles.value = data.total_files || 0
    totalBytes.value = data.total_bytes || 0
    selectedItem.value = null

    const nextState = items.value.length === 0 ? BROWSE_STATES.EMPTY : BROWSE_STATES.READY
    browseState.value = nextState
    syncAgentState({ browseState: nextState, currentPath: data.path })

    const targetHash = '#' + encodeURI(data.path)
    if (window.location.hash !== targetHash) {
      window.location.hash = encodeURI(data.path)
    }
  } catch (err) {
    if (err.name === 'AbortError' || reqId !== browseRequestId) return
    console.error('Failed to load path:', err)
    currentPath.value = path || '/'
    browseError.value = err.message || 'Failed to load directory'
    browseState.value = BROWSE_STATES.ERROR
    syncAgentState({ browseState: BROWSE_STATES.ERROR, currentPath: currentPath.value })
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
    refreshServerInfo()
  } catch (err) {
    alert('Failed to delete: ' + err.message)
  }
}

async function refreshServerInfo() {
  try {
    serverInfo.value = await fetchInfo()
  } catch (err) {
    console.error('Failed to load server info:', err)
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

// Coordinated Keyboard Shortcuts
function handleGlobalKeydown(e) {
  // Ctrl+K / Cmd+K -> Search Modal
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    if (activeModal.value === MODAL_STATES.SEARCH) {
      closeModal()
    } else {
      openModal(MODAL_STATES.SEARCH)
    }
    return
  }

  // Escape -> Dismiss active drag overlay or top-level modal
  if (e.key === 'Escape') {
    if (dragState.value === DRAG_STATES.DRAGGING) {
      resetDragState()
      return
    }
    if (activeModal.value !== MODAL_STATES.NONE) {
      e.preventDefault()
      closeModal()
      return
    }
  }

  // When any modal is open or user is typing in an input, block background workspace shortcuts
  if (activeModal.value !== MODAL_STATES.NONE) {
    return
  }

  if (['INPUT', 'TEXTAREA', 'SELECT'].includes(document.activeElement?.tagName)) {
    return
  }

  // Space -> QuickLook preview
  if (e.code === 'Space') {
    if (selectedItem.value && !selectedItem.value.is_dir) {
      e.preventDefault()
      openPreview(selectedItem.value)
    }
  }

  // Backspace -> Parent directory
  if (e.key === 'Backspace') {
    if (currentPath.value !== '/') {
      e.preventDefault()
      navigateTo(parentPath.value)
    }
  }
}

function handleHashChange() {
  const hashPath = decodeURI(window.location.hash.slice(1)) || '/'
  if (hashPath !== currentPath.value) {
    loadDirectory(hashPath)
  }
}

function handleStorageChange(e) {
  if (e.key === 'awdex-theme') {
    applyTheme(e.newValue !== 'light')
  }
}

onMounted(async () => {
  applyTheme(isDark.value)
  window.addEventListener('keydown', handleGlobalKeydown)
  window.addEventListener('hashchange', handleHashChange)
  window.addEventListener('storage', handleStorageChange)
  window.addEventListener('blur', resetDragState)
  window.addEventListener('dragend', resetDragState)

  refreshServerInfo()

  // Initial path from hash or root
  const initialPath = decodeURI(window.location.hash.slice(1)) || '/'
  loadDirectory(initialPath)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleGlobalKeydown)
  window.removeEventListener('hashchange', handleHashChange)
  window.removeEventListener('storage', handleStorageChange)
  window.removeEventListener('blur', resetDragState)
  window.removeEventListener('dragend', resetDragState)
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
  display: flex;
  flex-direction: column;
}

.main-content.has-audio-dock {
  padding-bottom: calc(68px + env(safe-area-inset-bottom, 0px));
}

.content-loading,
.content-error {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  padding: 24px;
  gap: 12px;
  color: var(--text-muted);
  font-size: 14px;
  text-align: center;
}

.error-icon {
  color: #ef4444;
  opacity: 0.85;
}

.error-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-main);
  margin: 0;
}

.error-desc {
  font-size: 13px;
  color: var(--text-subtle);
  max-width: 420px;
  margin: 0;
}

.error-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 6px;
}

.error-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 14px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-subtle);
  background-color: var(--bg-surface);
  color: var(--text-main);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.error-btn:hover {
  background-color: var(--bg-surface-hover);
}

.error-btn.primary {
  background-color: var(--primary);
  border-color: var(--primary);
  color: #ffffff;
}

.error-btn.primary:hover {
  background-color: var(--primary-hover);
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
