<template>
  <header class="navbar">
    <!-- Brand & Sidebar Toggle -->
    <div class="navbar-left">
      <button 
        class="icon-btn sidebar-toggle-btn" 
        @click="$emit('toggle-sidebar')" 
        :title="sidebarOpen ? 'Hide Tree' : 'Show Tree'"
      >
        <Sidebar :size="18" />
      </button>

      <div class="brand" @click="$emit('navigate', '/')">
        <FolderTree :size="20" class="brand-icon" />
        <span class="brand-title">{{ title || 'awdex' }}</span>
      </div>

      <!-- Breadcrumbs Trail -->
      <nav ref="breadcrumbsRef" class="breadcrumbs" aria-label="Breadcrumb">
        <ol class="breadcrumbs-list">
          <li v-for="(crumb, idx) in breadcrumbs" :key="crumb.path" class="breadcrumb-item">
            <span v-if="idx > 0" class="breadcrumb-separator">/</span>
            <button 
              class="breadcrumb-link" 
              :class="{ 'is-active': idx === breadcrumbs.length - 1 }"
              @click="$emit('navigate', crumb.path)"
            >
              {{ crumb.name === 'Root' ? '~' : crumb.name }}
            </button>
          </li>
        </ol>
      </nav>
    </div>

    <!-- Right Controls -->
    <div class="navbar-right">
      <!-- Search Trigger Pill -->
      <button class="search-trigger" @click="$emit('open-search')" title="Search files (Ctrl+K)">
        <Search :size="15" />
        <span class="search-label">Search library...</span>
        <kbd class="search-kbd">⌘K</kbd>
      </button>

      <!-- View Switcher -->
      <div class="view-switcher">
        <button 
          class="icon-btn" 
          :class="{ 'is-active': viewMode === 'details' }" 
          @click="$emit('update:viewMode', 'details')"
          title="Details View"
        >
          <List :size="17" />
        </button>
        <button 
          class="icon-btn" 
          :class="{ 'is-active': viewMode === 'grid' }" 
          @click="$emit('update:viewMode', 'grid')"
          title="Grid / Thumbnail View"
        >
          <LayoutGrid :size="17" />
        </button>
        <button 
          class="icon-btn hide-xs" 
          :class="{ 'is-active': viewMode === 'icons' }" 
          @click="$emit('update:viewMode', 'icons')"
          title="Compact Icons View"
        >
          <Grid :size="17" />
        </button>
      </div>

      <!-- Action Buttons -->
      <div class="action-buttons">
        <button 
          v-if="!readOnly" 
          class="btn btn-primary" 
          @click="$emit('open-upload')"
          title="Upload Files"
        >
          <Upload :size="15" />
          <span class="btn-text">Upload</span>
        </button>

        <button 
          class="icon-btn hide-xs" 
          @click="$emit('refresh')" 
          title="Refresh"
        >
          <RefreshCw :size="17" />
        </button>

        <button 
          class="icon-btn" 
          @click="$emit('toggle-theme')" 
          :title="isDark ? 'Light Mode' : 'Dark Mode'"
        >
          <Sun v-if="isDark" :size="17" />
          <Moon v-else :size="17" />
        </button>

        <button 
          class="icon-btn" 
          @click="$emit('open-info')" 
          title="Server Info"
        >
          <Info :size="17" />
        </button>
      </div>
    </div>
  </header>
</template>

<script setup>
import { ref, watch, nextTick } from 'vue'
import { 
  FolderTree, Sidebar, Search, List, LayoutGrid, Grid, 
  Upload, RefreshCw, Sun, Moon, Info 
} from '@lucide/vue'

const props = defineProps({
  title: String,
  breadcrumbs: {
    type: Array,
    default: () => []
  },
  viewMode: {
    type: String,
    default: 'details'
  },
  isDark: Boolean,
  sidebarOpen: Boolean,
  readOnly: Boolean
})

defineEmits([
  'navigate', 'toggle-sidebar', 'open-search', 'open-upload', 
  'refresh', 'toggle-theme', 'open-info', 'update:viewMode'
])

const breadcrumbsRef = ref(null)

watch(() => props.breadcrumbs, async () => {
  await nextTick()
  if (breadcrumbsRef.value) {
    breadcrumbsRef.value.scrollLeft = breadcrumbsRef.value.scrollWidth
  }
}, { deep: true })
</script>

<style scoped>
.navbar {
  height: 52px;
  background-color: var(--bg-surface);
  border-bottom: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 16px;
  gap: 8px;
  z-index: 20;
}

.navbar-left {
  display: flex;
  align-items: center;
  gap: 10px;
  overflow: hidden;
  flex: 1;
  min-width: 0;
}

.brand {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  font-weight: 700;
  font-size: 15px;
  color: var(--text-main);
  padding: 4px 8px;
  border-radius: var(--radius-sm);
  transition: background-color 0.15s;
  flex-shrink: 0;
}

.brand:hover {
  background-color: var(--bg-surface-hover);
}

.brand-icon {
  color: var(--primary);
}

.breadcrumbs {
  overflow-x: auto;
  scrollbar-width: none;
  flex: 1;
  min-width: 0;
}
.breadcrumbs::-webkit-scrollbar {
  display: none;
}

.breadcrumbs-list {
  display: flex;
  align-items: center;
  list-style: none;
  white-space: nowrap;
}

.breadcrumb-item {
  display: inline-flex;
  align-items: center;
}

.breadcrumb-separator {
  color: var(--text-subtle);
  margin: 0 3px;
  font-size: 13px;
}

.breadcrumb-link {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-muted);
  padding: 4px 6px;
  border-radius: var(--radius-sm);
  transition: all 0.15s;
}

.breadcrumb-link:hover {
  color: var(--text-main);
  background-color: var(--bg-surface-hover);
}

.breadcrumb-link.is-active {
  color: var(--text-main);
  font-weight: 600;
}

.navbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.search-trigger {
  background-color: var(--bg-app);
  border: 1px solid var(--border-subtle);
  padding: 6px 12px;
  border-radius: 20px;
  color: var(--text-muted);
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  transition: all 0.15s;
}

.search-trigger:hover {
  border-color: var(--border-strong);
  color: var(--text-main);
  background-color: var(--bg-surface-hover);
}

.search-label {
  display: inline-block;
}

.search-kbd {
  font-family: var(--font-mono);
  font-size: 10px;
  background-color: var(--bg-surface);
  border: 1px solid var(--border-strong);
  padding: 1px 5px;
  border-radius: 4px;
  color: var(--text-subtle);
}

.view-switcher, .action-buttons {
  display: flex;
  align-items: center;
  gap: 3px;
}

.icon-btn {
  width: 34px;
  height: 34px;
  border-radius: var(--radius-sm);
  color: var(--text-muted);
  flex-shrink: 0;
}

.icon-btn:hover {
  background-color: var(--bg-surface-hover);
  color: var(--text-main);
}

.icon-btn.is-active {
  background-color: var(--primary-subtle);
  color: var(--primary);
}

.btn {
  padding: 6px 12px;
  border-radius: var(--radius-sm);
  font-size: 13px;
  font-weight: 600;
  gap: 6px;
}

.btn-primary {
  background-color: var(--primary);
  color: #ffffff;
}

.btn-primary:hover {
  background-color: var(--primary-hover);
}

@media (max-width: 768px) {
  .navbar {
    padding: 0 10px;
    gap: 4px;
  }
  .navbar-left {
    gap: 6px;
  }
  .brand {
    padding: 4px;
  }
  .brand-title, .search-label, .search-kbd, .btn-text {
    display: none;
  }
  .search-trigger {
    padding: 7px;
    border-radius: var(--radius-sm);
  }
  .btn {
    padding: 7px 9px;
  }
}

@media (max-width: 480px) {
  .hide-xs {
    display: none;
  }
}
</style>
