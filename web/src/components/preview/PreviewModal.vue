<template>
  <div v-if="item" class="preview-backdrop" @click.self="$emit('close')">
    <div class="preview-container" :class="item.category">
      <!-- Header Bar -->
      <div class="preview-header">
        <div class="header-left">
          <span class="preview-filename" :title="item.name">{{ item.name }}</span>
          <span class="preview-filesize">{{ formatBytes(item.size) }}</span>
          <span v-if="isTranscoding" class="transcode-pill">Transcoding (H.264)</span>
        </div>

        <div class="header-right">
          <!-- Video Transcode Switch -->
          <button 
            v-if="item.category === 'video'" 
            class="header-btn" 
            :class="{ 'is-active': isTranscoding }" 
            @click="toggleTranscode"
            :title="isTranscoding ? 'Using FFmpeg Transcode (click to switch to Direct Stream)' : 'Using Direct Stream (click to force FFmpeg Transcode)'"
          >
            <Sliders :size="16" />
          </button>

          <!-- Text wrap toggle for code/text -->
          <button 
            v-if="item.category === 'code' || item.category === 'text'" 
            class="header-btn" 
            :class="{ 'is-active': wrapText }" 
            @click="wrapText = !wrapText"
            title="Toggle Word Wrap"
          >
            <WrapText :size="16" />
          </button>

          <!-- Markdown render toggle -->
          <button 
            v-if="isMarkdown" 
            class="header-btn" 
            :class="{ 'is-active': renderMarkdown }" 
            @click="renderMarkdown = !renderMarkdown"
            title="Toggle Markdown Render"
          >
            <BookOpen :size="16" />
          </button>

          <!-- Copy code button -->
          <button 
            v-if="textContent" 
            class="header-btn" 
            @click="copyText"
            :title="copied ? 'Copied!' : 'Copy to Clipboard'"
          >
            <Check v-if="copied" :size="16" class="text-green" />
            <Copy v-else :size="16" />
          </button>

          <!-- Image Zoom / Rotate Controls -->
          <template v-if="item.category === 'image'">
            <button class="header-btn hide-xs" @click="zoomIn" title="Zoom In"><ZoomIn :size="16" /></button>
            <button class="header-btn hide-xs" @click="zoomOut" title="Zoom Out"><ZoomOut :size="16" /></button>
            <button class="header-btn" @click="rotateImg" title="Rotate 90°"><RotateCw :size="16" /></button>
            <button class="header-btn hide-xs" @click="resetImage" title="Reset Zoom"><Maximize2 :size="16" /></button>
          </template>

          <!-- Direct Download -->
          <a :href="'/raw' + encodePath(item.path) + '?download=1'" class="header-btn" title="Download" download>
            <Download :size="16" />
          </a>

          <!-- Close Modal -->
          <button class="header-btn close-btn" @click="$emit('close')" title="Close (Esc)">
            <X :size="18" />
          </button>
        </div>
      </div>

      <!-- Preview Body -->
      <div 
        class="preview-body"
        @touchstart.passive="onTouchStart"
        @touchend.passive="onTouchEnd"
      >
        <!-- 1. IMAGE VIEWER -->
        <div v-if="item.category === 'image'" class="image-viewer">
          <button v-if="hasPrev" class="nav-arrow prev-arrow" @click="$emit('prev')" title="Previous (Left Arrow)">
            <ChevronLeft :size="28" />
          </button>

          <div class="image-stage" @wheel.prevent="onWheel">
            <img 
              :src="'/raw' + encodePath(item.path)" 
              class="preview-img" 
              :style="{ transform: `scale(${zoom}) rotate(${rotation}deg)` }"
              alt=""
            />
          </div>

          <button v-if="hasNext" class="nav-arrow next-arrow" @click="$emit('next')" title="Next (Right Arrow)">
            <ChevronRight :size="28" />
          </button>
        </div>

        <!-- 2. VIDEO PLAYER -->
        <div v-else-if="item.category === 'video'" class="video-viewer">
          <video 
            ref="videoRef" 
            controls 
            autoplay 
            playsinline 
            class="preview-video"
            :src="videoSrc"
            @error="onVideoError"
          >
            Your browser does not support the video tag.
          </video>
        </div>

        <!-- 3. AUDIO VIEWER -->
        <div v-else-if="item.category === 'audio'" class="audio-viewer">
          <div class="audio-card">
            <div class="audio-cover">
              <img v-if="item.has_thumb" :src="item.thumb_url" alt="" class="audio-cover-img" />
              <Music v-else :size="64" class="audio-placeholder-icon" />
            </div>
            <div class="audio-details">
              <h3 class="audio-title">{{ item.name }}</h3>
              <p class="audio-meta">{{ formatBytes(item.size) }}</p>
            </div>
            <audio controls autoplay class="audio-native-player" :src="'/raw' + encodePath(item.path)"></audio>
          </div>
        </div>

        <!-- 4. PDF VIEWER -->
        <div v-else-if="item.extension === '.pdf'" class="pdf-viewer">
          <iframe :src="'/raw' + encodePath(item.path)" class="pdf-iframe"></iframe>
        </div>

        <!-- 5. CODE / TEXT VIEWER -->
        <div v-else-if="item.category === 'code' || item.category === 'text'" class="text-viewer">
          <div v-if="loadingText" class="loading-spinner">Loading text...</div>
          
          <!-- Rendered Markdown -->
          <div 
            v-else-if="isMarkdown && renderMarkdown" 
            class="markdown-content" 
            v-html="renderedMarkdown"
          ></div>

          <!-- Highlighted Code / Text -->
          <pre v-else class="code-pre" :class="{ 'wrap-text': wrapText }"><code ref="codeRef" class="hljs" v-html="highlightedCode"></code></pre>
        </div>

        <!-- 6. ZIP ARCHIVE INSPECTOR -->
        <div v-else-if="item.category === 'archive' && item.extension === '.zip'" class="archive-viewer">
          <div v-if="loadingArchive" class="loading-spinner">Reading archive table of contents...</div>
          <table v-else class="archive-table">
            <thead>
              <tr>
                <th>File Path inside Archive</th>
                <th style="width: 120px">Size</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="entry in archiveEntries" :key="entry.name">
                <td class="archive-entry-name">
                  <Folder v-if="entry.is_dir" :size="15" class="folder-color" />
                  <File v-else :size="15" />
                  <span>{{ entry.name }}</span>
                </td>
                <td class="archive-entry-size">{{ entry.is_dir ? '-' : formatBytes(entry.size) }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- 7. GENERIC / BINARY FALLBACK -->
        <div v-else class="binary-viewer">
          <div class="binary-card">
            <File :size="64" class="binary-icon" />
            <h3 class="binary-name">{{ item.name }}</h3>
            <p class="binary-desc">{{ formatBytes(item.size) }} • {{ item.mime_type || 'Unknown Type' }}</p>
            <a :href="'/raw' + encodePath(item.path) + '?download=1'" class="btn btn-primary binary-download-btn" download>
              <Download :size="16" />
              <span>Download File</span>
            </a>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, computed, onMounted, onUnmounted } from 'vue'
import { 
  X, Download, ZoomIn, ZoomOut, RotateCw, Maximize2, 
  ChevronLeft, ChevronRight, Copy, Check, WrapText, BookOpen, 
  Music, File, Folder, Sliders 
} from '@lucide/vue'
import { formatBytes, encodePath } from '../../utils/formatters.js'
import { inspectArchive } from '../../utils/api.js'
import hljs from 'highlight.js'
import { marked } from 'marked'

const props = defineProps({
  item: Object,
  hasPrev: Boolean,
  hasNext: Boolean
})

const emit = defineEmits(['close', 'prev', 'next'])

// Image state
const zoom = ref(1)
const rotation = ref(0)

// Touch swipe state
let touchStartX = 0
let touchStartY = 0

function onTouchStart(e) {
  if (e.touches.length === 1) {
    touchStartX = e.touches[0].clientX
    touchStartY = e.touches[0].clientY
  }
}

function onTouchEnd(e) {
  if (zoom.value !== 1 || e.changedTouches.length !== 1) return
  const dx = e.changedTouches[0].clientX - touchStartX
  const dy = e.changedTouches[0].clientY - touchStartY
  if (Math.abs(dx) > 60 && Math.abs(dx) > Math.abs(dy) * 1.5) {
    if (dx > 0 && props.hasPrev) {
      emit('prev')
    } else if (dx < 0 && props.hasNext) {
      emit('next')
    }
  }
}

// Video state
const forceTranscode = ref(false)

const isNonNativeVideo = computed(() => {
  const ext = props.item?.extension?.toLowerCase()
  return ['.wmv', '.asf', '.avi', '.flv', '.vob', '.mpg', '.mpeg', '.ts', '.m2ts', '.mts', '.rm', '.rmvb', '.divx', '.3gp'].includes(ext)
})

const isTranscoding = computed(() => {
  return isNonNativeVideo.value || forceTranscode.value
})

const videoSrc = computed(() => {
  if (!props.item) return ''
  if (isTranscoding.value) {
    return '/api/transcode?path=' + encodeURIComponent(props.item.path)
  }
  return '/raw' + encodePath(props.item.path)
})

function toggleTranscode() {
  forceTranscode.value = !isTranscoding.value
}

function isNativeVideoFormat(ext) {
  const e = (ext || '').toLowerCase()
  return ['.mp4', '.m4v', '.webm', '.mov', '.ogg', '.ogv'].includes(e)
}

function onVideoError() {
  if (!isTranscoding.value) {
    if (!isNativeVideoFormat(props.item?.extension)) {
      console.log('Direct video playback failed for non-native video, falling back to FFmpeg transcode')
      forceTranscode.value = true
    } else {
      console.warn('Direct video playback error encountered on native format')
    }
  }
}

// Text/Code state
const textContent = ref('')
const loadingText = ref(false)
const wrapText = ref(true)
const renderMarkdown = ref(true)
const copied = ref(false)

// Archive state
const archiveEntries = ref([])
const loadingArchive = ref(false)

const isMarkdown = computed(() => {
  return props.item?.extension === '.md' || props.item?.extension === '.markdown'
})

const renderedMarkdown = computed(() => {
  if (!textContent.value) return ''
  return marked.parse(textContent.value)
})

const highlightedCode = computed(() => {
  if (!textContent.value) return ''
  try {
    const ext = props.item?.extension ? props.item.extension.slice(1) : ''
    if (ext && hljs.getLanguage(ext)) {
      return hljs.highlight(textContent.value, { language: ext }).value
    }
    return hljs.highlightAuto(textContent.value).value
  } catch {
    return textContent.value
  }
})

function zoomIn() { zoom.value = Math.min(zoom.value + 0.25, 4) }
function zoomOut() { zoom.value = Math.max(zoom.value - 0.25, 0.25) }
function rotateImg() { rotation.value = (rotation.value + 90) % 360 }
function resetImage() { zoom.value = 1; rotation.value = 0 }

function onWheel(e) {
  if (props.item?.category === 'image') {
    if (e.deltaY < 0) zoomIn()
    else zoomOut()
  }
}

async function copyText() {
  if (!textContent.value) return
  await navigator.clipboard.writeText(textContent.value)
  copied.value = true
  setTimeout(() => copied.value = false, 2000)
}

async function loadContent() {
  if (!props.item) return
  resetImage()
  forceTranscode.value = false

  if (props.item.category === 'code' || props.item.category === 'text') {
    loadingText.value = true
    try {
      const res = await fetch('/raw' + encodePath(props.item.path))
      textContent.value = await res.text()
    } catch (err) {
      textContent.value = 'Failed to load file contents: ' + err.message
    } finally {
      loadingText.value = false
    }
  } else if (props.item.category === 'archive' && props.item.extension === '.zip') {
    loadingArchive.value = true
    try {
      archiveEntries.value = await inspectArchive(props.item.path)
    } catch (err) {
      console.error('Failed to inspect zip:', err)
    } finally {
      loadingArchive.value = false
    }
  }
}

watch(() => props.item?.path, () => {
  loadContent()
}, { immediate: true })

function handleKeyDown(e) {
  if (e.key === 'Escape') emit('close')
  if (e.key === 'ArrowLeft' && props.hasPrev) emit('prev')
  if (e.key === 'ArrowRight' && props.hasNext) emit('next')
}

onMounted(() => {
  window.addEventListener('keydown', handleKeyDown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeyDown)
})
</script>

<style scoped>
.preview-backdrop {
  position: fixed;
  inset: 0;
  background-color: rgba(0, 0, 0, 0.85);
  backdrop-filter: blur(8px);
  z-index: 100;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}

.preview-container {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-xl);
  width: min(92vw, 1200px);
  height: 85dvh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: var(--shadow-xl);
}

.preview-header {
  height: 50px;
  padding: 0 16px;
  border-bottom: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: space-between;
  background-color: var(--bg-surface);
  flex-shrink: 0;
  gap: 8px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  flex: 1;
}

.preview-filename {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-main);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.preview-filesize {
  font-size: 12px;
  color: var(--text-subtle);
  font-family: var(--font-mono);
  flex-shrink: 0;
}

.transcode-pill {
  font-size: 11px;
  font-weight: 600;
  background-color: rgba(245, 158, 11, 0.2);
  color: #f59e0b;
  padding: 2px 8px;
  border-radius: 10px;
  white-space: nowrap;
  flex-shrink: 0;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.header-btn {
  width: 34px;
  height: 34px;
  border-radius: var(--radius-sm);
  color: var(--text-muted);
}
.header-btn:hover {
  background-color: var(--bg-surface-hover);
  color: var(--text-main);
}
.header-btn.is-active {
  background-color: var(--primary-subtle);
  color: var(--primary);
}
.close-btn:hover {
  color: #ef4444;
}

.text-green { color: #10b981; }

.preview-body {
  flex: 1;
  overflow: hidden;
  position: relative;
  display: flex;
  background-color: var(--bg-app);
}

/* Image Viewer */
.image-viewer {
  flex: 1;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

.image-stage {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: grab;
}

.preview-img {
  max-width: 92%;
  max-height: 92%;
  object-fit: contain;
  display: block;
  transition: transform 0.15s ease-out;
}

.nav-arrow {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  width: 42px;
  height: 42px;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(4px);
  color: white;
  z-index: 10;
}
.nav-arrow:hover {
  background: rgba(0, 0, 0, 0.8);
}
.prev-arrow { left: 12px; }
.next-arrow { right: 12px; }

/* Video Viewer */
.video-viewer {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #000;
}
.preview-video {
  max-width: 100%;
  max-height: 100%;
  display: block;
  outline: none;
}

/* Audio Viewer */
.audio-viewer {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
}
.audio-card {
  background: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  padding: 28px;
  border-radius: var(--radius-lg);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  width: min(380px, 100%);
  box-shadow: var(--shadow-lg);
}
.audio-cover {
  width: 160px;
  height: 160px;
  border-radius: var(--radius-md);
  overflow: hidden;
  background: var(--bg-app);
  display: flex;
  align-items: center;
  justify-content: center;
}
.audio-cover-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.audio-placeholder-icon {
  color: var(--text-subtle);
}
.audio-details {
  text-align: center;
  max-width: 100%;
}
.audio-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-main);
  word-break: break-word;
}
.audio-meta {
  font-size: 13px;
  color: var(--text-muted);
  margin-top: 4px;
}
.audio-native-player {
  width: 100%;
}

/* PDF Viewer */
.pdf-viewer {
  flex: 1;
  display: flex;
}
.pdf-iframe {
  width: 100%;
  height: 100%;
  border: none;
  display: block;
}

/* Text / Code Viewer */
.text-viewer {
  flex: 1;
  overflow: auto;
  overscroll-behavior: contain;
  padding: 20px;
  font-family: var(--font-mono);
  font-size: 13px;
}
.code-pre {
  margin: 0;
  white-space: pre;
}
.code-pre.wrap-text {
  white-space: pre-wrap;
  word-break: break-word;
}

/* Markdown Content */
.markdown-content {
  font-family: var(--font-sans);
  font-size: 15px;
  line-height: 1.6;
  max-width: 800px;
  margin: 0 auto;
  color: var(--text-main);
}
.markdown-content :deep(h1), 
.markdown-content :deep(h2), 
.markdown-content :deep(h3) {
  margin-top: 24px;
  margin-bottom: 12px;
  font-weight: 600;
}
.markdown-content :deep(p) {
  margin-bottom: 16px;
}
.markdown-content :deep(pre) {
  background: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  padding: 12px;
  border-radius: var(--radius-sm);
  overflow-x: auto;
  margin-bottom: 16px;
}
.markdown-content :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin-bottom: 16px;
}
.markdown-content :deep(th), 
.markdown-content :deep(td) {
  border: 1px solid var(--border-subtle);
  padding: 8px 12px;
}

/* Archive Viewer */
.archive-viewer {
  flex: 1;
  overflow: auto;
  overscroll-behavior: contain;
  padding: 16px;
}
.archive-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.archive-table th {
  text-align: left;
  padding: 8px 12px;
  border-bottom: 1px solid var(--border-subtle);
  color: var(--text-muted);
}
.archive-table td {
  padding: 6px 12px;
  border-bottom: 1px solid var(--border-subtle);
}
.archive-entry-name {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-main);
  word-break: break-all;
}
.archive-entry-size {
  color: var(--text-muted);
  font-family: var(--font-mono);
}
.folder-color {
  color: #f59e0b;
}

/* Binary / Fallback */
.binary-viewer {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
}
.binary-card {
  background: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  padding: 32px;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  width: min(380px, 100%);
  box-shadow: var(--shadow-lg);
}
.binary-icon {
  color: var(--text-subtle);
}
.binary-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-main);
  word-break: break-word;
}
.binary-desc {
  font-size: 13px;
  color: var(--text-muted);
}
.binary-download-btn {
  margin-top: 8px;
}

@media (max-width: 768px) {
  .preview-backdrop {
    padding: 0;
  }
  .preview-container {
    width: 100%;
    height: 100dvh;
    border-radius: 0;
    border: none;
  }
  .preview-header {
    height: calc(50px + env(safe-area-inset-top, 0px));
    padding: env(safe-area-inset-top, 0px) 10px 0 10px;
  }
  .preview-filesize, .hide-xs {
    display: none;
  }
  .text-viewer {
    padding: 12px;
  }
}
</style>
