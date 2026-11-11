<template>
  <div class="modal-backdrop" @click.self="handleClose">
    <div class="modal-card animate-fade-in" :data-upload-state="uploadState">
      <div class="modal-header">
        <h3 class="modal-title">Upload to {{ currentPath }}</h3>
        <button class="modal-close-btn" @click="handleClose">
          <X :size="18" />
        </button>
      </div>

      <div class="modal-body">
        <!-- Dropzone Area -->
        <div 
          class="dropzone-area" 
          :class="{ 'is-dragging': isDragging }"
          @dragover.prevent="isDragging = true"
          @dragleave.prevent="isDragging = false"
          @drop.prevent="onDrop"
          @click="triggerFileInput"
        >
          <UploadCloud :size="48" class="dropzone-icon" />
          <p class="dropzone-main">Drag and drop files here, or <span class="highlight">browse</span></p>
          <p class="dropzone-hint">Uploads stream directly to storage</p>
          <input 
            type="file" 
            ref="fileInputRef" 
            multiple 
            class="hidden-file-input" 
            @change="onFileSelected" 
          />
        </div>

        <!-- Selected Files List -->
        <div v-if="files.length > 0" class="file-queue-list">
          <div class="queue-header">
            <span>{{ files.length }} {{ files.length === 1 ? 'file' : 'files' }} selected ({{ formatBytes(totalBytes) }})</span>
            <button v-if="!uploading" class="clear-btn" @click="clearQueue">Clear</button>
          </div>

          <div class="queue-items">
            <div v-for="(f, idx) in files" :key="`${f.name}-${f.size}-${idx}`" class="queue-item">
              <span class="file-name" :title="f.name">{{ f.name }}</span>
              <div class="queue-item-right">
                <span class="file-size">{{ formatBytes(f.size) }}</span>
                <button v-if="!uploading" class="remove-file-btn" @click="removeFile(idx)" title="Remove from queue">
                  <X :size="13" />
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Upload Progress -->
        <div v-if="uploading" class="progress-section">
          <div class="progress-bar-container">
            <div class="progress-bar-fill" :style="{ width: progress + '%' }"></div>
          </div>
          <span class="progress-text">{{ progress }}% uploaded...</span>
        </div>

        <div v-if="errorMessage" class="error-banner">
          {{ errorMessage }}
        </div>
      </div>

      <div class="modal-footer">
        <button class="btn btn-secondary" @click="handleClose">
          {{ uploading ? 'Abort Upload' : 'Cancel' }}
        </button>
        <button 
          class="btn btn-primary" 
          :disabled="files.length === 0 || uploading"
          @click="startUpload"
        >
          {{ uploading ? 'Uploading...' : 'Upload Files' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onUnmounted } from 'vue'
import { X, UploadCloud } from '@lucide/vue'
import { formatBytes } from '../utils/formatters.js'
import { uploadFiles } from '../utils/api.js'
import { UPLOAD_STATES, syncAgentState } from '../utils/stateMachines.js'

const props = defineProps({
  currentPath: String,
  initialFiles: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['close', 'uploaded'])

const fileInputRef = ref(null)
const files = ref([])
const isDragging = ref(false)
const uploadState = ref(UPLOAD_STATES.IDLE)
const progress = ref(0)
const errorMessage = ref('')
let abortUploadFn = null

const uploading = computed(() => uploadState.value === UPLOAD_STATES.UPLOADING)

function setUploadState(next) {
  uploadState.value = next
  syncAgentState({ uploadState: next })
}

const totalBytes = computed(() => {
  return files.value.reduce((acc, f) => acc + f.size, 0)
})

function triggerFileInput() {
  if (!uploading.value) {
    fileInputRef.value?.click()
  }
}

function onFileSelected(e) {
  if (e.target.files) {
    addFiles(Array.from(e.target.files))
    e.target.value = ''
  }
}

function onDrop(e) {
  isDragging.value = false
  if (e.dataTransfer?.files) {
    addFiles(Array.from(e.dataTransfer.files))
  }
}

function addFiles(newFiles) {
  if (!newFiles || newFiles.length === 0) return
  const existingKeys = new Set(files.value.map(f => `${f.name}:${f.size}:${f.lastModified}`))
  const deduped = newFiles.filter(f => !existingKeys.has(`${f.name}:${f.size}:${f.lastModified}`))
  files.value = [...files.value, ...deduped]
  errorMessage.value = ''
  if (files.value.length > 0 && uploadState.value !== UPLOAD_STATES.UPLOADING) {
    setUploadState(UPLOAD_STATES.QUEUED)
  }
}

function removeFile(idx) {
  files.value.splice(idx, 1)
  if (files.value.length === 0) {
    setUploadState(UPLOAD_STATES.IDLE)
  }
}

function clearQueue() {
  files.value = []
  errorMessage.value = ''
  setUploadState(UPLOAD_STATES.IDLE)
}

function handleClose() {
  if (uploading.value && abortUploadFn) {
    abortUploadFn()
    abortUploadFn = null
  }
  emit('close')
}

watch(() => props.initialFiles, (init) => {
  if (init && init.length > 0) {
    addFiles(init)
  }
}, { immediate: true })

async function startUpload() {
  if (files.value.length === 0 || uploading.value) return
  setUploadState(UPLOAD_STATES.UPLOADING)
  progress.value = 0
  errorMessage.value = ''

  try {
    await uploadFiles(
      props.currentPath,
      files.value,
      (percent) => {
        progress.value = percent
      },
      (abortFn) => {
        abortUploadFn = abortFn
      }
    )
    setUploadState(UPLOAD_STATES.COMPLETED)
    emit('uploaded')
    emit('close')
  } catch (err) {
    errorMessage.value = err.message || 'Upload failed'
    setUploadState(UPLOAD_STATES.ERROR)
  } finally {
    abortUploadFn = null
  }
}

onUnmounted(() => {
  if (abortUploadFn) {
    abortUploadFn()
  }
  syncAgentState({ uploadState: UPLOAD_STATES.IDLE })
})
</script>

<style scoped>
.modal-backdrop {
  position: fixed;
  inset: 0;
  background-color: rgba(0, 0, 0, 0.65);
  backdrop-filter: blur(4px);
  z-index: 100;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}

.modal-card {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  width: 100%;
  max-width: 520px;
  overflow: hidden;
  box-shadow: var(--shadow-xl);
  display: flex;
  flex-direction: column;
}

.modal-header {
  height: 50px;
  padding: 0 16px;
  border-bottom: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.modal-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-main);
}

.modal-close-btn {
  width: 28px;
  height: 28px;
  border-radius: var(--radius-sm);
  color: var(--text-muted);
}
.modal-close-btn:hover {
  background-color: var(--bg-surface-hover);
  color: var(--text-main);
}

.modal-body {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.dropzone-area {
  border: 2px dashed var(--border-strong);
  border-radius: var(--radius-md);
  padding: 32px 16px;
  text-align: center;
  cursor: pointer;
  transition: all 0.15s;
  background-color: var(--bg-app);
}

.dropzone-area:hover, .dropzone-area.is-dragging {
  border-color: var(--primary);
  background-color: var(--primary-subtle);
}

.dropzone-icon {
  color: var(--primary);
  margin-bottom: 12px;
}

.dropzone-main {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-main);
}

.highlight {
  color: var(--primary);
  text-decoration: underline;
}

.dropzone-hint {
  font-size: 12px;
  color: var(--text-subtle);
  margin-top: 4px;
}

.hidden-file-input {
  display: none;
}

.file-queue-list {
  background-color: var(--bg-app);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  padding: 10px;
}

.queue-header {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
  margin-bottom: 8px;
}

.clear-btn {
  color: #ef4444;
  font-size: 11px;
}

.queue-items {
  max-height: 140px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.queue-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 12px;
  color: var(--text-main);
  gap: 8px;
}

.queue-item-right {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.remove-file-btn {
  color: var(--text-subtle);
  padding: 2px;
  border-radius: 3px;
}
.remove-file-btn:hover {
  color: #ef4444;
  background-color: var(--bg-surface-hover);
}

.file-name {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 300px;
}

.file-size {
  color: var(--text-subtle);
  font-family: var(--font-mono);
}

.progress-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.progress-bar-container {
  height: 6px;
  background-color: var(--border-subtle);
  border-radius: 3px;
  overflow: hidden;
}

.progress-bar-fill {
  height: 100%;
  background-color: var(--primary);
  transition: width 0.2s ease;
}

.progress-text {
  font-size: 11px;
  color: var(--text-subtle);
  text-align: right;
  font-family: var(--font-mono);
}

.error-banner {
  background-color: rgba(239, 68, 68, 0.1);
  border: 1px solid rgba(239, 68, 68, 0.3);
  color: #ef4444;
  font-size: 12px;
  padding: 8px 12px;
  border-radius: var(--radius-sm);
}

.modal-footer {
  height: 52px;
  padding: 0 16px;
  border-top: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  background-color: var(--bg-surface);
}

.btn {
  padding: 6px 14px;
  border-radius: var(--radius-sm);
  font-size: 13px;
  font-weight: 600;
}

.btn-secondary {
  background-color: var(--bg-surface-hover);
  color: var(--text-main);
}

.btn-primary {
  background-color: var(--primary);
  color: #fff;
}
.btn-primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
