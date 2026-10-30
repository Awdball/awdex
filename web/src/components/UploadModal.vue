<template>
  <div class="modal-backdrop" @click.self="$emit('close')">
    <div class="modal-card animate-fade-in">
      <div class="modal-header">
        <h3 class="modal-title">Upload to {{ currentPath }}</h3>
        <button class="modal-close-btn" @click="$emit('close')">
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
            <span>{{ files.length }} files selected ({{ formatBytes(totalBytes) }})</span>
            <button v-if="!uploading" class="clear-btn" @click="files = []">Clear</button>
          </div>

          <div class="queue-items">
            <div v-for="(f, idx) in files" :key="idx" class="queue-item">
              <span class="file-name" :title="f.name">{{ f.name }}</span>
              <span class="file-size">{{ formatBytes(f.size) }}</span>
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
        <button class="btn btn-secondary" @click="$emit('close')" :disabled="uploading">
          Cancel
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
import { ref, computed } from 'vue'
import { X, UploadCloud } from '@lucide/vue'
import { formatBytes } from '../utils/formatters.js'
import { uploadFiles } from '../utils/api.js'

const props = defineProps({
  currentPath: String
})

const emit = defineEmits(['close', 'uploaded'])

const fileInputRef = ref(null)
const files = ref([])
const isDragging = ref(false)
const uploading = ref(false)
const progress = ref(0)
const errorMessage = ref('')

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
  }
}

function onDrop(e) {
  isDragging.value = false
  if (e.dataTransfer?.files) {
    addFiles(Array.from(e.dataTransfer.files))
  }
}

function addFiles(newFiles) {
  files.value = [...files.value, ...newFiles]
}

async function startUpload() {
  if (files.value.length === 0 || uploading.value) return
  uploading.value = true
  progress.value = 0
  errorMessage.value = ''

  try {
    await uploadFiles(props.currentPath, files.value, (percent) => {
      progress.value = percent
    })
    emit('uploaded')
    emit('close')
  } catch (err) {
    errorMessage.value = err.message || 'Upload failed'
  } finally {
    uploading.value = false
  }
}
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
  font-size: 12px;
  color: var(--text-main);
}

.file-name {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 320px;
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
