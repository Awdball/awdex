<template>
  <div v-if="item" class="preview-backdrop" @click.self="$emit('close')">
    <div 
      class="preview-container" 
      :class="item.category"
      :data-preview-state="contentState"
      :data-video-state="item.category === 'video' ? videoState : undefined"
    >
      <!-- Header Bar -->
      <div class="preview-header">
        <div class="header-left">
          <span class="preview-filename" :title="item.name">{{ item.name }}</span>
          <span class="preview-filesize">{{ formatBytes(item.size) }}</span>
          <span v-if="item.category === 'video' && isTranscoding" class="transcode-pill">Transcoding (H.264)</span>
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
            v-if="textContent && contentState === CONTENT_STATES.READY" 
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

          <div v-if="contentState === CONTENT_STATES.ERROR" class="binary-card">
            <AlertCircle :size="48" class="binary-icon" />
            <h3 class="binary-name">Unable to render image</h3>
            <p class="binary-desc">{{ item.name }} ({{ formatBytes(item.size) }})</p>
            <a :href="'/raw' + encodePath(item.path) + '?download=1'" class="btn btn-primary binary-download-btn" download>
              <Download :size="16" />
              <span>Download Image</span>
            </a>
          </div>

          <div v-else class="image-stage" @wheel.prevent="onWheel">
            <img 
              :src="'/raw' + encodePath(item.path)" 
              class="preview-img" 
              :style="{ transform: `scale(${zoom}) rotate(${rotation}deg)` }"
              alt=""
              @load="setContentState(CONTENT_STATES.READY)"
              @error="setContentState(CONTENT_STATES.ERROR)"
            />
          </div>

          <button v-if="hasNext" class="nav-arrow next-arrow" @click="$emit('next')" title="Next (Right Arrow)">
            <ChevronRight :size="28" />
          </button>
        </div>

        <!-- 2. VIDEO PLAYER -->
        <div v-else-if="item.category === 'video'" class="video-viewer">
          <div v-if="videoState === VIDEO_STATES.ERROR" class="binary-card">
            <AlertCircle :size="48" class="binary-icon" />
            <h3 class="binary-name">Video playback unavailable</h3>
            <p class="binary-desc">Neither direct browser decoding nor FFmpeg live transcoding succeeded for this stream.</p>
            <div style="display:flex;gap:8px;flex-wrap:wrap;justify-content:center;margin-top:8px;">
              <button class="btn" style="border:1px solid var(--border-subtle);background:var(--bg-app);color:var(--text-main);" @click="retryVideoDirect">
                Retry Direct Stream
              </button>
              <a :href="'/raw' + encodePath(item.path) + '?download=1'" class="btn btn-primary" download>
                <Download :size="16" />
                <span>Download Video</span>
              </a>
            </div>
          </div>

          <video 
            v-else
            ref="videoRef" 
            controls 
            autoplay 
            playsinline 
            class="preview-video"
            :src="videoSrc"
            @loadeddata="setContentState(CONTENT_STATES.READY)"
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
          <iframe :src="'/raw' + encodePath(item.path)" class="pdf-iframe" @load="setContentState(CONTENT_STATES.READY)"></iframe>
        </div>

        <!-- 5. CODE / TEXT VIEWER -->
        <div v-else-if="item.category === 'code' || item.category === 'text'" class="text-viewer">
          <div v-if="contentState === CONTENT_STATES.LOADING" class="loading-spinner">Loading text...</div>

          <div v-else-if="contentState === CONTENT_STATES.ERROR" class="loading-spinner">
            {{ contentError || 'Failed to load file contents.' }}
          </div>
          
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
          <div v-if="contentState === CONTENT_STATES.LOADING" class="loading-spinner">Reading archive table of contents...</div>
          <div v-else-if="contentState === CONTENT_STATES.ERROR" class="loading-spinner">{{ contentError || 'Failed to inspect archive.' }}</div>
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
  Music, File, Folder, Sliders, AlertCircle
} from '@lucide/vue'
import { formatBytes, encodePath } from '../../utils/formatters.js'
import { inspectArchive } from '../../utils/api.js'
import { VIDEO_STATES, CONTENT_STATES, syncAgentState } from '../../utils/stateMachines.js'
import hljs from 'highlight.js'
import { marked } from 'marked'

const props = defineProps({
  item: Object,
  hasPrev: Boolean,
  hasNext: Boolean
})

const emit = defineEmits(['close', 'prev', 'next'])

// Preview Content State Machine
const contentState = ref(CONTENT_STATES.IDLE)
const contentError = ref('')
let loadSeq = 0

function setContentState(next, err = '') {
  contentState.value = next
  contentError.value = err
  syncAgentState({ previewState: next })
}

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

// Video Playback Fallback State Machine: 'direct' -> 'transcoding' -> 'error'
const videoState = ref(VIDEO_STATES.DIRECT)

function setVideoState(next) {
  videoState.value = next
  syncAgentState({ videoState: next })
}

const isNonNativeVideo = computed(() => {
  const ext = props.item?.extension?.toLowerCase()
  return ['.wmv', '.asf', '.avi', '.flv', '.vob', '.mpg', '.mpeg', '.ts', '.m2ts', '.mts', '.rm', '.rmvb', '.divx', '.3gp'].includes(ext)
})

const isTranscoding = computed(() => {
  return videoState.value === VIDEO_STATES.TRANSCODING
})

const videoSrc = computed(() => {
  if (!props.item) return ''
  if (videoState.value === VIDEO_STATES.TRANSCODING) {
    return '/api/transcode?path=' + encodeURIComponent(props.item.path)
  }
  return '/raw' + encodePath(props.item.path)
})

function toggleTranscode() {
  if (videoState.value === VIDEO_STATES.TRANSCODING) {
    setVideoState(VIDEO_STATES.DIRECT)
  } else {
    setVideoState(VIDEO_STATES.TRANSCODING)
  }
}

function retryVideoDirect() {
  setVideoState(VIDEO_STATES.DIRECT)
  setContentState(CONTENT_STATES.LOADING)
}

function onVideoError() {
  if (videoState.value === VIDEO_STATES.DIRECT) {
    // Any direct playback failure (including .mp4/.mkv/.mov with unsupported HEVC/AC3 codecs)
    // automatically transitions once to FFmpeg live transcoding.
    setVideoState(VIDEO_STATES.TRANSCODING)
  } else if (videoState.value === VIDEO_STATES.TRANSCODING) {
    setVideoState(VIDEO_STATES.ERROR)
    setContentState(CONTENT_STATES.ERROR, 'Video stream failed')
  }
}

// Text/Code state
const textContent = ref('')
const wrapText = ref(true)
const renderMarkdown = ref(true)
const copied = ref(false)

// Archive state
const archiveEntries = ref([])

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
  const seq = ++loadSeq
  resetImage()
  textContent.value = ''
  archiveEntries.value = []

  if (props.item.category === 'video') {
    setVideoState(isNonNativeVideo.value ? VIDEO_STATES.TRANSCODING : VIDEO_STATES.DIRECT)
    setContentState(CONTENT_STATES.LOADING)
    return
  }

  setVideoState(null)

  if (props.item.category === 'code' || props.item.category === 'text') {
    setContentState(CONTENT_STATES.LOADING)
    try {
      const res = await fetch('/raw' + encodePath(props.item.path))
      if (!res.ok) {
        throw new Error((await res.text()).trim() || `HTTP ${res.status}`)
      }
      const txt = await res.text()
      if (seq !== loadSeq) return
      textContent.value = txt
      setContentState(CONTENT_STATES.READY)
    } catch (err) {
      if (seq !== loadSeq) return
      setContentState(CONTENT_STATES.ERROR, 'Failed to load file contents: ' + err.message)
    }
  } else if (props.item.category === 'archive' && props.item.extension === '.zip') {
    setContentState(CONTENT_STATES.LOADING)
    try {
      const entries = await inspectArchive(props.item.path)
      if (seq !== loadSeq) return
      archiveEntries.value = entries || []
      setContentState(CONTENT_STATES.READY)
    } catch (err) {
      if (seq !== loadSeq) return
      console.error('Failed to inspect zip:', err)
      setContentState(CONTENT_STATES.ERROR, 'Failed to inspect archive: ' + err.message)
    }
  } else if (props.item.category === 'image' || props.item.extension === '.pdf') {
    setContentState(CONTENT_STATES.LOADING)
  } else {
    setContentState(CONTENT_STATES.READY)
  }
}

watch(() => props.item?.path, () => {
  loadContent()
}, { immediate: true })

function handleKeyDown(e) {
  if (e.key === 'ArrowLeft' && props.hasPrev) {
    e.preventDefault()
    emit('prev')
  } else if (e.key === 'ArrowRight' && props.hasNext) {
    e.preventDefault()
    emit('next')
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeyDown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeyDown)
  syncAgentState({ previewState: CONTENT_STATES.IDLE, videoState: null })
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

.loading-spinner {
  padding: 40px 20px;
  text-align: center;
  color: var(--text-muted);
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

.btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 14px;
  border-radius: var(--radius-sm);
  font-size: 13px;
  font-weight: 600;
  text-decoration: none;
  cursor: pointer;
}
.btn-primary {
  background-color: var(--primary);
  color: #ffffff;
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
