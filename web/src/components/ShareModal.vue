<template>
  <dialog ref="dialogRef" class="share-dialog animate-fade-in" @close="$emit('close')" @click="onDialogClick">
    <div class="share-modal-inner" @click.stop>
      <div class="modal-header">
        <div class="header-title-row">
          <Share2 :size="18" class="header-icon" />
          <h3 class="modal-title">{{ item ? 'Share Item' : 'Share Manager & Tunnel Engine' }}</h3>
        </div>
        <button class="modal-close-btn" @click="closeDialog" title="Close">
          <X :size="18" />
        </button>
      </div>

      <!-- Mode Tabs -->
      <div class="modal-tabs">
        <button 
          v-if="item" 
          class="tab-btn" 
          :class="{ 'is-active': activeTab === 'create' }" 
          @click="activeTab = 'create'"
        >
          Create Link
        </button>
        <button 
          class="tab-btn" 
          :class="{ 'is-active': activeTab === 'manage' }" 
          @click="activeTab = 'manage'"
        >
          Active Shares ({{ shares.length }})
        </button>
        <button 
          class="tab-btn" 
          :class="{ 'is-active': activeTab === 'tunnel' }" 
          @click="activeTab = 'tunnel'"
        >
          Tunnel Engine
          <span class="tunnel-dot" :class="tunnel.running ? 'is-online' : 'is-idle'"></span>
        </button>
      </div>

      <!-- TAB 1: CREATE SHARE LINK -->
      <div v-if="activeTab === 'create' && item" class="modal-body">
        <div class="target-card">
          <Folder v-if="item.is_dir" :size="22" style="color: #f59e0b; flex-shrink: 0;" />
          <File v-else :size="22" style="color: var(--primary); flex-shrink: 0;" />
          <div class="target-info">
            <div class="target-name">{{ item.name }}</div>
            <div class="target-sub">{{ item.is_dir ? 'Folder (.ZIP Download & File List)' : formatBytes(item.size) }} • {{ item.path }}</div>
          </div>
        </div>

        <div v-if="createdShare" class="created-result-box">
          <div class="result-label">Public Share Link Ready ({{ createdShare.provider }})</div>
          <div class="url-copy-row">
            <input type="text" readonly :value="fullShareURL(createdShare.public_url)" class="url-input" @focus="$event.target.select()" />
            <button class="btn-primary copy-btn" @click="copyLink(createdShare.public_url, 'new')">
              <Check v-if="copiedId === 'new'" :size="15" />
              <Copy v-else :size="15" />
              <span>{{ copiedId === 'new' ? 'Copied!' : 'Copy' }}</span>
            </button>
          </div>
          <div v-if="warningMsg" class="warning-banner">{{ warningMsg }}</div>
          <div class="result-meta">
            <span>Isolated Share Port :{{ tunnel.share_port || 8081 }}</span>
            <span>•</span>
            <a :href="fullShareURL(createdShare.public_url)" target="_blank" rel="noopener" class="preview-link">Open Share Page ↗</a>
          </div>
        </div>

        <form v-else class="share-form" @submit.prevent="handleCreate">
          <div class="form-row">
            <div class="form-group">
              <label>Expires After</label>
              <select v-model.number="form.expiresInHours" class="form-select">
                <option :value="1">1 Hour</option>
                <option :value="6">6 Hours</option>
                <option :value="24">24 Hours</option>
                <option :value="168">7 Days</option>
                <option :value="720">30 Days</option>
                <option :value="0">Never Expires</option>
              </select>
            </div>

            <div class="form-group">
              <label>Max Downloads</label>
              <select v-model.number="form.maxDownloads" class="form-select">
                <option :value="0">Unlimited</option>
                <option :value="1">1 (Burn after reading)</option>
                <option :value="5">5 Downloads</option>
                <option :value="25">25 Downloads</option>
                <option :value="100">100 Downloads</option>
              </select>
            </div>
          </div>

          <div class="form-group">
            <label>Optional Password Protection</label>
            <input 
              type="password" 
              v-model="form.password" 
              placeholder="Leave blank for direct access without password" 
              class="form-input"
            />
          </div>

          <div class="engine-summary-bar">
            <span>Outbound Engine: <strong>{{ providerLabel(settings.provider) }}</strong></span>
            <button type="button" class="link-btn" @click="activeTab = 'tunnel'">Change Engine</button>
          </div>

          <div v-if="errorMsg" class="error-banner">{{ errorMsg }}</div>

          <button type="submit" class="btn-primary submit-btn" :disabled="creating">
            <Share2 :size="16" />
            <span>{{ creating ? 'Starting Tunnel & Creating Link...' : 'Generate Shareable Link' }}</span>
          </button>
        </form>
      </div>

      <!-- TAB 2: ACTIVE SHARES -->
      <div v-else-if="activeTab === 'manage'" class="modal-body">
        <div class="tunnel-status-strip" :class="{ 'is-running': tunnel.running }">
          <div class="strip-left">
            <span class="tunnel-dot" :class="tunnel.running ? 'is-online' : 'is-idle'"></span>
            <div>
              <div class="strip-title">
                {{ tunnel.running ? `Outbound Tunnel Active (${providerLabel(tunnel.provider)})` : `Tunnel Idle (${providerLabel(settings.provider)})` }}
              </div>
              <div class="strip-sub">
                {{ tunnel.running ? tunnel.public_base_url : 'Spins up automatically when a share is active; closes when empty.' }}
              </div>
            </div>
          </div>
          <button v-if="tunnel.running && settings.provider !== 'custom'" class="btn-outline-sm" @click="handleTunnelAction('stop')">
            Stop Tunnel
          </button>
          <button v-else-if="shares.length > 0 && !tunnel.running && settings.provider !== 'custom'" class="btn-outline-sm" @click="handleTunnelAction('start')">
            Start Tunnel
          </button>
        </div>

        <div v-if="shares.length === 0" class="empty-shares">
          <p>No active share links.</p>
          <span>Click the Share icon on any file or folder to generate an external link.</span>
        </div>

        <div v-else class="shares-list">
          <div v-for="s in shares" :key="s.id" class="share-row-card">
            <div class="share-row-top">
              <div class="share-item-title">
                <Lock v-if="s.has_password" :size="13" class="lock-icon" title="Password Protected" />
                <strong>{{ s.name }}</strong>
                <span class="share-badge">{{ s.provider }}</span>
              </div>
              <button class="revoke-btn" @click="handleDelete(s.id)" title="Revoke Share">
                <Trash2 :size="14" />
                <span>Revoke</span>
              </button>
            </div>

            <div class="share-meta-line">
              <span>Downloads: {{ s.downloads }}{{ s.max_downloads > 0 ? ` / ${s.max_downloads}` : '' }}</span>
              <span>•</span>
              <span>{{ s.expires_at ? 'Expires ' + formatDate(s.expires_at) : 'Never expires' }}</span>
            </div>

            <div class="url-copy-row compact">
              <input type="text" readonly :value="fullShareURL(s.public_url)" class="url-input" @focus="$event.target.select()" />
              <button class="btn-primary copy-btn" @click="copyLink(s.public_url, s.id)">
                <Check v-if="copiedId === s.id" :size="14" />
                <Copy v-else :size="14" />
                <span>{{ copiedId === s.id ? 'Copied' : 'Copy' }}</span>
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- TAB 3: TUNNEL ENGINE CONFIG -->
      <div v-else-if="activeTab === 'tunnel'" class="modal-body">
        <p class="engine-intro">
          Share links are served from an isolated share-only listener on port <code>:{{ tunnel.share_port || 8081 }}</code>. Unshared files and the main <code>awdex</code> UI are never exposed to the tunnel.
        </p>

        <div class="provider-cards">
          <label class="provider-card" :class="{ 'is-selected': configForm.provider === 'cloudflared' }">
            <input type="radio" v-model="configForm.provider" value="cloudflared" />
            <div class="provider-card-body">
              <div class="provider-name">Cloudflare Quick Tunnel (Default)</div>
              <div class="provider-desc">Zero-config outbound ephemeral tunnel (<code>*.trycloudflare.com</code>). Requires no account or port forwarding. Auto-starts on share and auto-stops when all shares expire.</div>
            </div>
          </label>

          <label class="provider-card" :class="{ 'is-selected': configForm.provider === 'zrok' }">
            <input type="radio" v-model="configForm.provider" value="zrok" />
            <div class="provider-card-body">
              <div class="provider-name">zrok (OpenZiti Zero-Trust Public Share)</div>
              <div class="provider-desc">Spins up an outbound OpenZiti public share (<code>*.share.zrok.io</code>) pointed at port <code>:{{ tunnel.share_port || 8081 }}</code>.</div>
            </div>
          </label>

          <label class="provider-card" :class="{ 'is-selected': configForm.provider === 'custom' }">
            <input type="radio" v-model="configForm.provider" value="custom" />
            <div class="provider-card-body">
              <div class="provider-name">Custom URL / Tailscale Funnel</div>
              <div class="provider-desc">Use your own Tailscale Funnel, Pangolin, frp, or rathole tunnel pointed at port <code>:{{ tunnel.share_port || 8081 }}</code>.</div>
            </div>
          </label>
        </div>

        <div v-if="configForm.provider === 'zrok'" class="form-group">
          <label>zrok Account Token {{ settings.has_zrok_token ? '(Saved ✓ — leave blank to keep current)' : '' }}</label>
          <input 
            type="password" 
            v-model="configForm.zrokToken" 
            placeholder="Enter zrok enable token (or set AWDEX_ZROK_TOKEN)" 
            class="form-input"
          />
        </div>

        <div v-if="configForm.provider === 'custom'" class="form-group">
          <label>Public Base URL (Routed to port :{{ tunnel.share_port || 8081 }})</label>
          <input 
            type="url" 
            v-model="configForm.publicBaseURL" 
            placeholder="https://awdex-share.your-tailnet.ts.net" 
            class="form-input"
          />
        </div>

        <div v-if="configSavedMsg" class="saved-banner">{{ configSavedMsg }}</div>
        <div v-if="warningMsg" class="warning-banner">{{ warningMsg }}</div>

        <button class="btn-primary submit-btn" :disabled="savingConfig" @click="handleSaveConfig">
          <span>{{ savingConfig ? 'Saving...' : 'Save Tunnel Engine Settings' }}</span>
        </button>
      </div>
    </div>
  </dialog>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { Share2, X, Folder, File, Copy, Check, Trash2, Lock } from '@lucide/vue'
import { fetchShares, createShareLink, deleteShareLink, updateShareConfig } from '../utils/api.js'
import { formatBytes, formatDate } from '../utils/formatters.js'

const props = defineProps({
  item: {
    type: Object,
    default: null
  }
})

const emit = defineEmits(['close', 'updated'])

const dialogRef = ref(null)
const activeTab = ref(props.item ? 'create' : 'manage')
const shares = ref([])
const tunnel = ref({ running: false, provider: 'cloudflared', public_base_url: '', share_port: 8081 })
const settings = ref({ provider: 'cloudflared', public_base_url: '', has_zrok_token: false })

const creating = ref(false)
const createdShare = ref(null)
const errorMsg = ref('')
const warningMsg = ref('')
const copiedId = ref('')
const savingConfig = ref(false)
const configSavedMsg = ref('')

const form = reactive({
  expiresInHours: 24,
  maxDownloads: 0,
  password: ''
})

const configForm = reactive({
  provider: 'cloudflared',
  publicBaseURL: '',
  zrokToken: ''
})

onMounted(async () => {
  if (dialogRef.value && !dialogRef.value.open) {
    dialogRef.value.showModal()
  }
  await loadState()
})

function closeDialog() {
  if (dialogRef.value && dialogRef.value.open) {
    dialogRef.value.close()
  }
  emit('close')
}

function onDialogClick(e) {
  if (e.target === dialogRef.value) {
    closeDialog()
  }
}

async function loadState() {
  try {
    const data = await fetchShares()
    shares.value = data.shares || []
    tunnel.value = data.tunnel || tunnel.value
    settings.value = data.settings || settings.value
    configForm.provider = settings.value.provider || 'cloudflared'
    configForm.publicBaseURL = settings.value.public_base_url || ''
    emit('updated', shares.value.length)
  } catch (err) {
    errorMsg.value = err.message
  }
}

function providerLabel(p) {
  if (p === 'zrok') return 'zrok'
  if (p === 'custom') return 'Custom / Funnel'
  return 'Cloudflare Quick Tunnel'
}

function fullShareURL(url) {
  if (!url) return ''
  if (url.startsWith('http://') || url.startsWith('https://')) return url
  return window.location.origin + url
}

async function handleCreate() {
  if (!props.item) return
  creating.value = true
  errorMsg.value = ''
  warningMsg.value = ''
  try {
    const res = await createShareLink({
      path: props.item.path,
      expires_in_hours: form.expiresInHours,
      max_downloads: form.maxDownloads,
      password: form.password
    })
    createdShare.value = res.share
    if (res.warning) warningMsg.value = res.warning
    await loadState()
  } catch (err) {
    errorMsg.value = err.message
  } finally {
    creating.value = false
  }
}

async function handleDelete(id) {
  try {
    const res = await deleteShareLink(id)
    shares.value = res.shares || []
    tunnel.value = res.tunnel || tunnel.value
    if (createdShare.value && createdShare.value.id === id) {
      createdShare.value = null
    }
    emit('updated', shares.value.length)
  } catch (err) {
    errorMsg.value = err.message
  }
}

async function handleSaveConfig() {
  savingConfig.value = true
  configSavedMsg.value = ''
  warningMsg.value = ''
  try {
    const res = await updateShareConfig({
      provider: configForm.provider,
      public_base_url: configForm.publicBaseURL,
      zrok_token: configForm.zrokToken
    })
    shares.value = res.shares || []
    tunnel.value = res.tunnel || tunnel.value
    settings.value = res.settings || settings.value
    configForm.zrokToken = ''
    if (res.warning) warningMsg.value = res.warning
    configSavedMsg.value = 'Tunnel engine settings saved.'
  } catch (err) {
    warningMsg.value = err.message
  } finally {
    savingConfig.value = false
  }
}

async function handleTunnelAction(action) {
  warningMsg.value = ''
  try {
    const res = await updateShareConfig({ action })
    shares.value = res.shares || []
    tunnel.value = res.tunnel || tunnel.value
    if (res.warning) warningMsg.value = res.warning
  } catch (err) {
    warningMsg.value = err.message
  }
}

async function copyLink(url, id) {
  const full = fullShareURL(url)
  try {
    await navigator.clipboard.writeText(full)
    copiedId.value = id
    setTimeout(() => {
      if (copiedId.value === id) copiedId.value = ''
    }, 2000)
  } catch {
    // Fallback
  }
}
</script>

<style scoped>
dialog.share-dialog {
  margin: auto;
  padding: 0;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background-color: var(--bg-surface);
  color: var(--text-main);
  width: calc(100% - 24px);
  max-width: 540px;
  max-height: 88dvh;
  box-shadow: var(--shadow-xl);
  overflow: auto;
  overscroll-behavior: contain;
}

dialog.share-dialog:modal {
  overscroll-behavior: contain;
}

dialog.share-dialog::backdrop {
  background-color: rgba(0, 0, 0, 0.65);
  backdrop-filter: blur(4px);
  overflow: hidden;
  overscroll-behavior: contain;
}

.share-modal-inner {
  display: flex;
  flex-direction: column;
  width: 100%;
}

.modal-header {
  height: 50px;
  padding: 0 16px;
  border-bottom: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.header-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.header-icon {
  color: var(--primary);
}

.modal-title {
  font-size: 15px;
  font-weight: 600;
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

.modal-tabs {
  display: flex;
  border-bottom: 1px solid var(--border-subtle);
  background-color: var(--bg-app);
  padding: 0 12px;
  gap: 4px;
}

.tab-btn {
  padding: 10px 12px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-muted);
  border-bottom: 2px solid transparent;
  gap: 6px;
}

.tab-btn.is-active {
  color: var(--primary);
  border-bottom-color: var(--primary);
  font-weight: 600;
}

.tunnel-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
}
.tunnel-dot.is-online { background-color: #10b981; box-shadow: 0 0 6px #10b981; }
.tunnel-dot.is-idle { background-color: var(--text-subtle); }

.modal-body {
  padding: 18px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.target-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  background-color: var(--bg-app);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
}

.target-info {
  min-width: 0;
  flex: 1;
}

.target-name {
  font-size: 14px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.target-sub {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 2px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.share-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

@media (max-width: 480px) {
  .form-row {
    grid-template-columns: 1fr;
  }
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-group label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
}

.form-select, .form-input {
  background-color: var(--bg-app);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  padding: 9px 11px;
  color: var(--text-main);
  font-size: 13px;
  width: 100%;
}

.form-select:focus, .form-input:focus {
  border-color: var(--primary);
}

.engine-summary-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
  color: var(--text-muted);
  background-color: var(--bg-app);
  padding: 8px 12px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-subtle);
}

.link-btn {
  color: var(--primary);
  font-size: 12px;
  font-weight: 600;
}

.btn-primary {
  background-color: var(--primary);
  color: #fff;
  font-weight: 600;
  border-radius: var(--radius-sm);
  padding: 10px 16px;
  font-size: 13px;
  gap: 8px;
}
.btn-primary:hover:not(:disabled) {
  background-color: var(--primary-hover);
}
.btn-primary:disabled {
  opacity: 0.6;
  cursor: wait;
}

.submit-btn {
  width: 100%;
}

.created-result-box {
  background-color: var(--bg-app);
  border: 1px solid var(--primary);
  border-radius: var(--radius-md);
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.result-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--primary);
}

.url-copy-row {
  display: flex;
  gap: 8px;
}

.url-input {
  flex: 1;
  min-width: 0;
  background-color: var(--bg-surface);
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-sm);
  padding: 8px 10px;
  color: var(--text-main);
  font-family: var(--font-mono);
  font-size: 12px;
}

.copy-btn {
  padding: 8px 12px;
  flex-shrink: 0;
}

.result-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--text-muted);
}

.preview-link {
  color: var(--primary);
  text-decoration: none;
  font-weight: 500;
}

.tunnel-status-strip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px;
  border-radius: var(--radius-md);
  background-color: var(--bg-app);
  border: 1px solid var(--border-subtle);
}

.strip-left {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.strip-title {
  font-size: 13px;
  font-weight: 600;
}

.strip-sub {
  font-size: 11px;
  color: var(--text-muted);
  word-break: break-all;
}

.btn-outline-sm {
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-sm);
  padding: 6px 10px;
  font-size: 12px;
  color: var(--text-main);
  flex-shrink: 0;
}
.btn-outline-sm:hover {
  background-color: var(--bg-surface-hover);
}

.empty-shares {
  text-align: center;
  padding: 32px 16px;
  color: var(--text-muted);
  font-size: 13px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.shares-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.share-row-card {
  background-color: var(--bg-app);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.share-row-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.share-item-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.lock-icon {
  color: #f59e0b;
  flex-shrink: 0;
}

.share-badge {
  font-size: 10px;
  text-transform: uppercase;
  background-color: var(--primary-subtle);
  color: var(--primary);
  padding: 1px 6px;
  border-radius: 4px;
  font-weight: 600;
}

.revoke-btn {
  color: #ef4444;
  font-size: 12px;
  gap: 4px;
  padding: 4px 8px;
  border-radius: 4px;
  flex-shrink: 0;
}
.revoke-btn:hover {
  background-color: rgba(239, 68, 68, 0.12);
}

.share-meta-line {
  display: flex;
  gap: 8px;
  font-size: 11px;
  color: var(--text-muted);
}

.engine-intro {
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.5;
}

.provider-cards {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.provider-card {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-subtle);
  background-color: var(--bg-app);
  cursor: pointer;
}

.provider-card.is-selected {
  border-color: var(--primary);
  background-color: var(--primary-subtle);
}

.provider-card input {
  margin-top: 3px;
}

.provider-name {
  font-size: 13px;
  font-weight: 600;
}

.provider-desc {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 2px;
  line-height: 1.4;
}

.error-banner, .warning-banner {
  background-color: rgba(239, 68, 68, 0.14);
  color: #f87171;
  padding: 8px 12px;
  border-radius: var(--radius-sm);
  font-size: 12px;
}

.saved-banner {
  background-color: rgba(16, 185, 129, 0.14);
  color: #10b981;
  padding: 8px 12px;
  border-radius: var(--radius-sm);
  font-size: 12px;
}
</style>
