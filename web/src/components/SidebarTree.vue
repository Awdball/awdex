<template>
  <aside class="sidebar-tree" :class="{ 'is-collapsed': !isOpen }">
    <div class="sidebar-header">
      <span class="sidebar-title">FOLDERS</span>
      <button class="collapse-btn" @click="$emit('toggle')" title="Toggle sidebar">
        <ChevronLeft :size="16" />
      </button>
    </div>

    <div class="tree-content">
      <!-- Root Node -->
      <div 
        class="tree-node-item root-item" 
        :class="{ 'is-selected': currentPath === '/' }"
        @click="$emit('navigate', '/')"
      >
        <FolderRoot :size="16" class="node-icon root-icon" />
        <span class="node-name">Root</span>
      </div>

      <!-- Recursive folder tree -->
      <TreeNodeItem 
        v-for="child in rootChildren" 
        :key="child.path" 
        :node="child" 
        :current-path="currentPath"
        @navigate="$emit('navigate', $event)" 
      />
    </div>
  </aside>
</template>

<script setup>
import { ref, onMounted, watch } from 'vue'
import { ChevronLeft, FolderRoot } from '@lucide/vue'
import TreeNodeItem from './TreeNodeItem.vue'
import { fetchTree } from '../utils/api.js'

const props = defineProps({
  isOpen: Boolean,
  currentPath: String
})

defineEmits(['navigate', 'toggle'])

const rootChildren = ref([])

async function loadRoot() {
  try {
    rootChildren.value = await fetchTree('/')
  } catch (err) {
    console.error('Failed to load root tree nodes:', err)
  }
}

onMounted(() => {
  loadRoot()
})

// Reload root tree if navigated back to root
watch(() => props.currentPath, (newVal) => {
  if (newVal === '/') {
    loadRoot()
  }
})
</script>

<style scoped>
.sidebar-tree {
  width: 260px;
  background-color: var(--bg-sidebar);
  border-right: 1px solid var(--border-subtle);
  display: flex;
  flex-direction: column;
  transition: width 0.2s cubic-bezier(0.16, 1, 0.3, 1), margin-left 0.2s cubic-bezier(0.16, 1, 0.3, 1), transform 0.22s cubic-bezier(0.16, 1, 0.3, 1);
  flex-shrink: 0;
  height: 100%;
  max-height: 100%;
  min-height: 0;
  overflow: hidden;
  user-select: none;
}

.sidebar-tree.is-collapsed {
  width: 0;
  margin-left: -260px;
  overflow: hidden;
}

@media (max-width: 768px) {
  .sidebar-tree {
    position: absolute;
    inset-block: 0;
    inset-inline-start: 0;
    z-index: 40;
    width: min(280px, 82vw);
    margin-left: 0;
    box-shadow: var(--shadow-xl);
    transform: translateX(0);
  }

  .sidebar-tree.is-collapsed {
    width: min(280px, 82vw);
    margin-left: 0;
    transform: translateX(-105%);
    box-shadow: none;
  }
}

.sidebar-header {
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 12px;
  border-bottom: 1px solid var(--border-subtle);
  flex-shrink: 0;
}

.sidebar-title {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.05em;
  color: var(--text-subtle);
}

.collapse-btn {
  width: 28px;
  height: 28px;
  color: var(--text-muted);
  border-radius: var(--radius-sm);
}
.collapse-btn:hover {
  background-color: var(--bg-surface-hover);
  color: var(--text-main);
}

.tree-content {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  overscroll-behavior: contain;
  padding: 8px 6px;
}

.tree-node-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: 13px;
  color: var(--text-muted);
  transition: all 0.12s;
}

.tree-node-item:hover {
  background-color: var(--bg-surface-hover);
  color: var(--text-main);
}

.tree-node-item.is-selected {
  background-color: var(--primary-subtle);
  color: var(--primary);
  font-weight: 600;
}

.root-icon {
  color: var(--primary);
}
</style>
