<template>
  <div class="modal-backdrop" @click.self="$emit('close')">
    <div
      class="info-modal animate-fade-in"
      :data-index-state="info?.index_state || 'unknown'"
    >
      <div class="modal-header">
        <h3 class="modal-title">System Information</h3>
        <button class="modal-close-btn" @click="$emit('close')">
          <X :size="18" />
        </button>
      </div>

      <div class="modal-body" v-if="info">
        <div class="info-section">
          <h4 class="section-title">Core</h4>
          <div class="info-row">
            <span class="info-label">Title</span>
            <span class="info-value">{{ info.title }}</span>
          </div>
          <div class="info-row">
            <span class="info-label">Version</span>
            <span class="info-value">v{{ info.version }}</span>
          </div>
          <div class="info-row">
            <span class="info-label">Database</span>
            <span class="info-value badge-db">{{ info.database_type }}</span>
          </div>
          <div class="info-row">
            <span class="info-label">Mode</span>
            <span class="info-value">{{ info.read_only ? 'Read-Only' : 'Read/Write (Uploads Enabled)' }}</span>
          </div>
        </div>

        <div class="info-section">
          <h4 class="section-title">Library Index</h4>
          <div class="info-row">
            <span class="info-label">Indexed Files</span>
            <span class="info-value font-mono">{{ info.indexed_files.toLocaleString() }}</span>
          </div>
          <div class="info-row">
            <span class="info-label">Indexed Folders</span>
            <span class="info-value font-mono">{{ info.indexed_dirs.toLocaleString() }}</span>
          </div>
          <div class="info-row">
            <span class="info-label">Total Size</span>
            <span class="info-value font-mono">{{ formatBytes(info.indexed_bytes) }}</span>
          </div>
          <div class="info-row">
            <span class="info-label">Crawler Status</span>
            <span class="info-value badge-status" :class="info.index_state">{{ info.index_state }}</span>
          </div>
          <div v-if="info.index_status?.last_duration_ms" class="info-row">
            <span class="info-label">Last Crawl Duration</span>
            <span class="info-value font-mono">{{ info.index_status.last_duration_ms }} ms</span>
          </div>
          <div v-if="info.index_status?.last_error" class="info-row">
            <span class="info-label">Last Crawler Error</span>
            <span class="info-value text-error">{{ info.index_status.last_error }}</span>
          </div>
        </div>

        <div class="info-section">
          <h4 class="section-title">Capabilities</h4>
          <div class="info-row">
            <span class="info-label">FFmpeg (Video Previews & Scrubs)</span>
            <span class="info-value" :class="info.ffmpeg_present ? 'text-green' : 'text-gray'">
              {{ info.ffmpeg_present ? 'Available' : 'Not installed' }}
            </span>
          </div>
          <div class="info-row">
            <span class="info-label">Poppler (PDF Thumbnails)</span>
            <span class="info-value" :class="info.poppler_present ? 'text-green' : 'text-gray'">
              {{ info.poppler_present ? 'Available' : 'Not installed' }}
            </span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { X } from '@lucide/vue'
import { formatBytes } from '../utils/formatters.js'

defineProps({
  info: Object
})

defineEmits(['close'])
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

.info-modal {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  width: 100%;
  max-width: 460px;
  box-shadow: var(--shadow-xl);
  display: flex;
  flex-direction: column;
  overflow: hidden;
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
  gap: 20px;
}

.section-title {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.05em;
  color: var(--text-subtle);
  margin-bottom: 8px;
  text-transform: uppercase;
}

.info-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px 0;
  font-size: 13px;
}

.info-label {
  color: var(--text-muted);
}

.info-value {
  color: var(--text-main);
  font-weight: 500;
}

.badge-db {
  text-transform: uppercase;
  background-color: var(--primary-subtle);
  color: var(--primary);
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
}

.badge-status {
  text-transform: capitalize;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11px;
}
.badge-status.ready { background: rgba(16, 185, 129, 0.15); color: #10b981; }
.badge-status.indexing { background: rgba(245, 158, 11, 0.15); color: #f59e0b; }
.badge-status.error { background: rgba(239, 68, 68, 0.15); color: #ef4444; }
.badge-status.stopping, .badge-status.idle { background: rgba(148, 163, 184, 0.15); color: var(--text-muted); }

.font-mono { font-family: var(--font-mono); }
.text-green { color: #10b981; }
.text-gray { color: var(--text-subtle); }
.text-error { color: #ef4444; font-size: 12px; }
</style>
