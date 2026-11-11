<template>
  <div v-if="track" class="audio-dock-player animate-slide-down" :data-player-state="playerState" :data-shuffle="isShuffle">
    <!-- Hidden Native Audio Element -->
    <audio 
      ref="audioRef" 
      :src="'/raw' + encodePath(track.path)" 
      @timeupdate="onTimeUpdate"
      @loadedmetadata="onLoadedMetadata"
      @play="onNativePlay"
      @playing="onNativePlaying"
      @pause="onNativePause"
      @waiting="onNativeWaiting"
      @error="onNativeError"
      @ended="onEnded"
      autoplay
    ></audio>

    <!-- Track Info -->
    <div class="dock-left">
      <div class="dock-thumb">
        <img v-if="track.has_thumb && !thumbFailed" :src="track.thumb_url" alt="" class="dock-thumb-img" @error="thumbFailed = true" />
        <Music v-else :size="20" class="dock-placeholder" />
      </div>
      <div class="dock-info">
        <span class="dock-title" :title="track.name">{{ track.name }}</span>
        <span class="dock-artist">
          <template v-if="playerState === AUDIO_STATES.ERROR">Playback error</template>
          <template v-else>{{ formatBytes(track.size) }}</template>
        </span>
      </div>
    </div>

    <!-- Playback Controls & Scrubber -->
    <div class="dock-center">
      <div class="dock-controls">
        <button class="dock-btn hide-mobile" :class="{ 'is-active': isShuffle }" @click="isShuffle = !isShuffle" title="Shuffle">
          <Shuffle :size="15" />
        </button>
        <button class="dock-btn" @click="$emit('prev')" title="Previous Track">
          <SkipBack :size="17" />
        </button>
        <button class="dock-btn play-btn" @click="togglePlay" :title="isPlaying ? 'Pause' : 'Play'">
          <Pause v-if="isPlaying" :size="18" />
          <Play v-else :size="18" />
        </button>
        <button class="dock-btn" @click="emitNext" title="Next Track">
          <SkipForward :size="17" />
        </button>
        <button class="dock-btn hide-mobile" :class="{ 'is-active': isLoop }" @click="isLoop = !isLoop" title="Repeat Track">
          <Repeat :size="15" />
        </button>
      </div>

      <div class="dock-progress-row">
        <span class="dock-time">{{ formatTime(currentTime) }}</span>
        <input 
          type="range" 
          min="0" 
          :max="duration || 100" 
          v-model="currentTime" 
          class="dock-slider" 
          @input="seek"
        />
        <span class="dock-time">{{ formatTime(duration) }}</span>
      </div>
    </div>

    <!-- Volume & Dismiss -->
    <div class="dock-right">
      <div class="volume-control">
        <button class="dock-btn" @click="toggleMute">
          <VolumeX v-if="volume === 0" :size="17" />
          <Volume2 v-else :size="17" />
        </button>
        <input 
          type="range" 
          min="0" 
          max="1" 
          step="0.05" 
          v-model="volume" 
          class="dock-slider volume-slider" 
          @input="setVolume"
        />
      </div>

      <button class="dock-btn close-btn" @click="$emit('close')" title="Close Player">
        <X :size="18" />
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { 
  Play, Pause, SkipBack, SkipForward, Repeat, Shuffle, 
  Volume2, VolumeX, Music, X 
} from '@lucide/vue'
import { formatBytes, encodePath } from '../utils/formatters.js'
import { AUDIO_STATES, syncAgentState } from '../utils/stateMachines.js'

const props = defineProps({
  track: Object
})

const emit = defineEmits(['close', 'next', 'prev'])

const audioRef = ref(null)
const playerState = ref(AUDIO_STATES.LOADING)
const thumbFailed = ref(false)
const currentTime = ref(0)
const duration = ref(0)
const volume = ref(0.8)
const prevVolume = ref(0.8)
const isLoop = ref(false)
const isShuffle = ref(false)

const isPlaying = computed(() => playerState.value === AUDIO_STATES.PLAYING)

function setPlayerState(next) {
  playerState.value = next
  syncAgentState({ audioState: next })
}

function togglePlay() {
  if (!audioRef.value) return
  if (audioRef.value.paused) {
    audioRef.value.play().catch(() => {
      setPlayerState(AUDIO_STATES.PAUSED)
    })
  } else {
    audioRef.value.pause()
  }
}

function onNativePlay() {
  setPlayerState(AUDIO_STATES.PLAYING)
}

function onNativePlaying() {
  setPlayerState(AUDIO_STATES.PLAYING)
}

function onNativePause() {
  if (audioRef.value && !audioRef.value.ended) {
    setPlayerState(AUDIO_STATES.PAUSED)
  }
}

function onNativeWaiting() {
  setPlayerState(AUDIO_STATES.LOADING)
}

function onNativeError() {
  setPlayerState(AUDIO_STATES.ERROR)
}

function onTimeUpdate() {
  if (audioRef.value) {
    currentTime.value = audioRef.value.currentTime
  }
}

function onLoadedMetadata() {
  if (audioRef.value) {
    duration.value = audioRef.value.duration
    audioRef.value.volume = volume.value
    audioRef.value.play().then(() => {
      setPlayerState(AUDIO_STATES.PLAYING)
    }).catch(() => {
      setPlayerState(AUDIO_STATES.PAUSED)
    })
  }
}

function emitNext() {
  emit('next', { shuffle: isShuffle.value })
}

function onEnded() {
  if (isLoop.value) {
    if (audioRef.value) {
      audioRef.value.currentTime = 0
      audioRef.value.play()
    }
  } else {
    emitNext()
  }
}

function seek(e) {
  const time = parseFloat(e.target.value)
  if (audioRef.value) {
    audioRef.value.currentTime = time
    currentTime.value = time
  }
}

function setVolume() {
  if (audioRef.value) {
    audioRef.value.volume = volume.value
  }
}

function toggleMute() {
  if (volume.value > 0) {
    prevVolume.value = volume.value
    volume.value = 0
  } else {
    volume.value = prevVolume.value || 0.8
  }
  setVolume()
}

function formatTime(secs) {
  if (isNaN(secs) || secs === undefined) return '0:00'
  const m = Math.floor(secs / 60)
  const s = Math.floor(secs % 60)
  return `${m}:${s < 10 ? '0' : ''}${s}`
}

watch(() => props.track?.path, () => {
  currentTime.value = 0
  duration.value = 0
  thumbFailed.value = false
  setPlayerState(AUDIO_STATES.LOADING)
}, { immediate: true })
</script>

<style scoped>
.audio-dock-player {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  min-height: 64px;
  padding: 6px 20px calc(6px + env(safe-area-inset-bottom, 0px));
  background-color: var(--bg-surface);
  border-top: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  box-shadow: var(--shadow-lg);
  z-index: 50;
}

.dock-left {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 260px;
  min-width: 0;
}

.dock-thumb {
  width: 42px;
  height: 42px;
  border-radius: var(--radius-sm);
  background-color: var(--bg-app);
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  flex-shrink: 0;
}

.dock-thumb-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.dock-placeholder {
  color: var(--text-subtle);
}

.dock-info {
  min-width: 0;
  flex: 1;
}

.dock-title {
  display: block;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-main);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.dock-artist {
  display: block;
  font-size: 11px;
  color: var(--text-subtle);
  margin-top: 2px;
}

.dock-center {
  flex: 1;
  max-width: 600px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.dock-controls {
  display: flex;
  align-items: center;
  gap: 10px;
}

.dock-btn {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  color: var(--text-muted);
}
.dock-btn:hover {
  color: var(--text-main);
  background-color: var(--bg-surface-hover);
}
.dock-btn.is-active {
  color: var(--primary);
}

.play-btn {
  background-color: var(--primary);
  color: #fff !important;
}
.play-btn:hover {
  background-color: var(--primary-hover);
}

.dock-progress-row {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 8px;
}

.dock-time {
  font-size: 11px;
  font-family: var(--font-mono);
  color: var(--text-subtle);
  width: 38px;
  text-align: center;
}

.dock-slider {
  flex: 1;
  height: 4px;
  accent-color: var(--primary);
  cursor: pointer;
}

.dock-right {
  display: flex;
  align-items: center;
  gap: 14px;
  width: 180px;
  justify-content: flex-end;
}

.volume-control {
  display: flex;
  align-items: center;
  gap: 6px;
}

.volume-slider {
  width: 70px;
}

.close-btn:hover {
  color: #ef4444;
}

@media (max-width: 768px) {
  .audio-dock-player {
    padding: 6px 12px calc(6px + env(safe-area-inset-bottom, 0px));
  }
  .dock-left {
    width: auto;
    flex: 1;
  }
  .dock-center {
    flex: 0 0 auto;
  }
  .dock-right {
    width: auto;
  }
  .volume-control,
  .dock-progress-row,
  .hide-mobile {
    display: none;
  }
}
</style>
