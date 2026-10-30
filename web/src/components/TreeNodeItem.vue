<template>
  <div class="tree-node-wrapper">
    <div 
      class="tree-node-row" 
      :class="{ 'is-selected': currentPath === node.path }"
      @click="$emit('navigate', node.path)"
    >
      <!-- Expand/Collapse Chevron -->
      <button 
        v-if="node.has_children" 
        class="toggle-chevron" 
        :class="{ 'is-expanded': isExpanded }"
        @click.stop="toggleExpand"
      >
        <ChevronRight :size="14" />
      </button>
      <span v-else class="chevron-placeholder"></span>

      <!-- Folder Icon -->
      <FolderOpen v-if="isExpanded" :size="16" class="folder-icon is-open" />
      <Folder v-else :size="16" class="folder-icon" />

      <!-- Folder Name -->
      <span class="node-label" :title="node.name">{{ node.name }}</span>

      <!-- Count Badge -->
      <span v-if="node.sub_count > 0" class="sub-count-badge">{{ node.sub_count }}</span>
    </div>

    <!-- Children container -->
    <div v-if="isExpanded && children.length > 0" class="tree-children">
      <TreeNodeItem 
        v-for="child in children" 
        :key="child.path" 
        :node="child" 
        :current-path="currentPath"
        @navigate="$emit('navigate', $event)" 
      />
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import { ChevronRight, Folder, FolderOpen } from '@lucide/vue'
import { fetchTree } from '../utils/api.js'

const props = defineProps({
  node: Object,
  currentPath: String
})

defineEmits(['navigate'])

const isExpanded = ref(false)
const children = ref([])
const loaded = ref(false)

async function toggleExpand() {
  if (!isExpanded.value) {
    if (!loaded.value) {
      try {
        children.value = await fetchTree(props.node.path)
        loaded.value = true
      } catch (err) {
        console.error('Failed to load sub tree:', err)
      }
    }
    isExpanded.value = true
  } else {
    isExpanded.value = false
  }
}

// Auto-expand if currentPath is a child of this node
watch(() => props.currentPath, (newVal) => {
  if (newVal && newVal.startsWith(props.node.path + '/') && !isExpanded.value) {
    toggleExpand()
  }
}, { immediate: true })
</script>

<style scoped>
.tree-node-wrapper {
  display: flex;
  flex-direction: column;
}

.tree-node-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 6px;
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: 13px;
  color: var(--text-muted);
  transition: all 0.12s;
}

.tree-node-row:hover {
  background-color: var(--bg-surface-hover);
  color: var(--text-main);
}

.tree-node-row.is-selected {
  background-color: var(--primary-subtle);
  color: var(--primary);
  font-weight: 600;
}

.toggle-chevron {
  width: 18px;
  height: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-subtle);
  transition: transform 0.15s ease;
  border-radius: 3px;
}
.toggle-chevron:hover {
  color: var(--text-main);
  background-color: var(--bg-surface-active);
}
.toggle-chevron.is-expanded {
  transform: rotate(90deg);
}

.chevron-placeholder {
  width: 18px;
}

.folder-icon {
  color: #f59e0b; /* Amber */
  flex-shrink: 0;
}
.folder-icon.is-open {
  color: #d97706;
}

.node-label {
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.sub-count-badge {
  font-size: 10px;
  font-family: var(--font-mono);
  background-color: var(--border-subtle);
  color: var(--text-subtle);
  padding: 1px 5px;
  border-radius: 10px;
  margin-left: auto;
}

.tree-children {
  padding-left: 16px;
  border-left: 1px solid var(--border-subtle);
  margin-left: 9px;
}
</style>
