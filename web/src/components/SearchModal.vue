<template>
  <div class="modal-backdrop" @click.self="$emit('close')">
    <div class="search-modal animate-fade-in">
      <!-- Search Input Bar -->
      <div class="search-input-header">
        <Search :size="18" class="search-icon" />
        <input 
          ref="searchInputRef" 
          type="text" 
          v-model="query" 
          placeholder="Search all files and folders across library..."
          class="search-main-input"
          @input="onSearchInput"
          @keydown.esc="$emit('close')"
        />
        <button class="search-close-btn" @click="$emit('close')">
          <X :size="18" />
        </button>
      </div>

      <!-- Category Filter Pills -->
      <div class="category-pills">
        <button 
          v-for="cat in categories" 
          :key="cat.id" 
          class="cat-pill" 
          :class="{ 'is-active': selectedCategory === cat.id }"
          @click="selectCategory(cat.id)"
        >
          {{ cat.label }}
        </button>
      </div>

      <!-- Results List -->
      <div class="search-results">
        <div v-if="loading" class="search-status">Searching index...</div>
        <div v-else-if="!query && results.length === 0" class="search-status">
          Type to search files by name, path, or extension
        </div>
        <div v-else-if="query && results.length === 0" class="search-status">
          No matches found for "{{ query }}"
        </div>

        <div v-else class="results-list">
          <div 
            v-for="item in results" 
            :key="item.path" 
            class="result-row"
            @click="onSelectResult(item)"
          >
            <div class="result-thumb-box">
              <img v-if="item.has_thumb" :src="item.thumb_url" alt="" class="result-thumb-img" />
              <Folder v-else-if="item.is_dir" :size="18" class="folder-color" />
              <File v-else :size="18" class="file-color" />
            </div>

            <div class="result-info">
              <span class="result-name">{{ item.name }}</span>
              <span class="result-path">{{ item.path }}</span>
            </div>

            <div class="result-meta">
              <span v-if="!item.is_dir" class="result-size">{{ formatBytes(item.size) }}</span>
              <span class="result-cat">{{ item.category }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { Search, X, Folder, File } from '@lucide/vue'
import { fetchSearch } from '../utils/api.js'
import { formatBytes } from '../utils/formatters.js'

const emit = defineEmits(['close', 'select-item', 'navigate'])

const searchInputRef = ref(null)
const query = ref('')
const selectedCategory = ref('')
const results = ref([])
const loading = ref(false)
let debounceTimer = null

const categories = [
  { id: '', label: 'All' },
  { id: 'video', label: 'Video' },
  { id: 'audio', label: 'Audio' },
  { id: 'image', label: 'Images' },
  { id: 'document', label: 'Docs' },
  { id: 'code', label: 'Code' },
  { id: 'archive', label: 'Archives' }
]

function selectCategory(catId) {
  selectedCategory.value = catId
  performSearch()
}

function onSearchInput() {
  clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    performSearch()
  }, 200)
}

async function performSearch() {
  if (!query.value.trim() && !selectedCategory.value) {
    results.value = []
    return
  }

  loading.value = true
  try {
    const data = await fetchSearch(query.value, selectedCategory.value, '/', 50, 0)
    results.value = data.items || []
  } catch (err) {
    console.error('Search failed:', err)
  } finally {
    loading.value = false
  }
}

function onSelectResult(item) {
  if (item.is_dir) {
    emit('navigate', item.path)
    emit('close')
  } else {
    emit('select-item', item)
    emit('close')
  }
}

onMounted(() => {
  searchInputRef.value?.focus()
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
  align-items: flex-start;
  justify-content: center;
  padding-top: 10vh;
}

.search-modal {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-xl);
  width: 100%;
  max-width: 680px;
  max-height: 70vh;
  box-shadow: var(--shadow-xl);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.search-input-header {
  height: 56px;
  padding: 0 16px;
  border-bottom: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  gap: 12px;
}

.search-icon {
  color: var(--primary);
}

.search-main-input {
  flex: 1;
  border: none;
  background: transparent;
  font-size: 15px;
  color: var(--text-main);
}

.search-close-btn {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-sm);
  color: var(--text-muted);
}
.search-close-btn:hover {
  background-color: var(--bg-surface-hover);
  color: var(--text-main);
}

.category-pills {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 16px;
  border-bottom: 1px solid var(--border-subtle);
  background-color: var(--bg-app);
  overflow-x: auto;
}

.cat-pill {
  font-size: 12px;
  font-weight: 500;
  padding: 4px 10px;
  border-radius: 14px;
  background-color: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  color: var(--text-muted);
}
.cat-pill:hover {
  border-color: var(--border-strong);
  color: var(--text-main);
}
.cat-pill.is-active {
  background-color: var(--primary);
  border-color: var(--primary);
  color: white;
}

.search-results {
  flex: 1;
  overflow-y: auto;
  padding: 8px 0;
  min-height: 200px;
}

.search-status {
  padding: 40px;
  text-align: center;
  font-size: 13px;
  color: var(--text-subtle);
}

.results-list {
  display: flex;
  flex-direction: column;
}

.result-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 16px;
  cursor: pointer;
  transition: background-color 0.12s;
}

.result-row:hover {
  background-color: var(--bg-surface-hover);
}

.result-thumb-box {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-sm);
  background-color: var(--bg-app);
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  flex-shrink: 0;
}

.result-thumb-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.folder-color { color: #f59e0b; }
.file-color { color: var(--text-subtle); }

.result-info {
  flex: 1;
  min-width: 0;
}

.result-name {
  display: block;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-main);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.result-path {
  display: block;
  font-size: 11px;
  color: var(--text-subtle);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-top: 1px;
}

.result-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.result-size {
  font-size: 11px;
  font-family: var(--font-mono);
  color: var(--text-subtle);
}

.result-cat {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  background-color: var(--border-subtle);
  color: var(--text-muted);
  padding: 1px 5px;
  border-radius: 3px;
}
</style>
