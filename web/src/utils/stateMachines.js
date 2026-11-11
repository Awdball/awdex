/**
 * Explicit State Machine Definitions & Observability Helpers for awdex.
 *
 * Elevates:
 * - UX: Eliminates impossible UI states, race conditions, and silent failures.
 * - DX: Centralizes state invariants, valid transitions, and human-readable state constants.
 * - AX: Publishes a deterministic, inspectable state graph on `window.__AWDEX_STATE__`
 *       and mirrors top-level states to root `data-awdex-*` DOM attributes.
 */

export const BROWSE_STATES = Object.freeze({
  IDLE: 'idle',
  LOADING: 'loading',
  READY: 'ready',
  EMPTY: 'empty',
  ERROR: 'error'
})

export const MODAL_STATES = Object.freeze({
  NONE: 'none',
  PREVIEW: 'preview',
  UPLOAD: 'upload',
  SEARCH: 'search',
  SHARE: 'share',
  INFO: 'info'
})

export const DRAG_STATES = Object.freeze({
  IDLE: 'idle',
  DRAGGING: 'dragging'
})

export const VIDEO_STATES = Object.freeze({
  DIRECT: 'direct',
  TRANSCODING: 'transcoding',
  ERROR: 'error'
})

export const CONTENT_STATES = Object.freeze({
  IDLE: 'idle',
  LOADING: 'loading',
  READY: 'ready',
  ERROR: 'error'
})

export const AUDIO_STATES = Object.freeze({
  IDLE: 'idle',
  LOADING: 'loading',
  PLAYING: 'playing',
  PAUSED: 'paused',
  ERROR: 'error'
})

export const UPLOAD_STATES = Object.freeze({
  IDLE: 'idle',
  QUEUED: 'queued',
  UPLOADING: 'uploading',
  COMPLETED: 'completed',
  ERROR: 'error'
})

export const SEARCH_STATES = Object.freeze({
  IDLE: 'idle',
  SEARCHING: 'searching',
  RESULTS: 'results',
  EMPTY: 'empty',
  ERROR: 'error'
})

export const TREE_NODE_STATES = Object.freeze({
  COLLAPSED: 'collapsed',
  LOADING: 'loading',
  EXPANDED: 'expanded',
  ERROR: 'error'
})

const globalAgentSnapshot = {
  browseState: BROWSE_STATES.IDLE,
  currentPath: '/',
  activeModal: MODAL_STATES.NONE,
  dragState: DRAG_STATES.IDLE,
  audioState: AUDIO_STATES.IDLE,
  previewState: CONTENT_STATES.IDLE,
  videoState: null,
  uploadState: UPLOAD_STATES.IDLE,
  searchState: SEARCH_STATES.IDLE,
  updatedAt: Date.now()
}

if (typeof window !== 'undefined') {
  window.__AWDEX_STATE__ = globalAgentSnapshot
}

/**
 * Synchronizes state machine transitions to `window.__AWDEX_STATE__` and root DOM dataset
 * so automated agents and developer tools can inspect live application state deterministically.
 */
export function syncAgentState(patch) {
  if (!patch || typeof patch !== 'object') return
  Object.assign(globalAgentSnapshot, patch, { updatedAt: Date.now() })
  if (typeof document !== 'undefined' && document.documentElement) {
    const ds = document.documentElement.dataset
    if (patch.browseState !== undefined) ds.awdexBrowse = patch.browseState
    if (patch.activeModal !== undefined) ds.awdexModal = patch.activeModal
    if (patch.currentPath !== undefined) ds.awdexPath = patch.currentPath
    if (patch.audioState !== undefined) ds.awdexAudio = patch.audioState
  }
}
