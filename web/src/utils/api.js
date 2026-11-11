const API_BASE = ''

export async function fetchBrowse(path = '/', options = {}) {
  const res = await fetch(`${API_BASE}/api/browse?path=${encodeURIComponent(path)}`, {
    signal: options.signal
  })
  if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`)
  return await res.json()
}

export async function fetchTree(path = '/', options = {}) {
  const res = await fetch(`${API_BASE}/api/tree?path=${encodeURIComponent(path)}`, {
    signal: options.signal
  })
  if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`)
  return await res.json()
}

export async function fetchSearch(query, category = '', path = '/', limit = 50, offset = 0, options = {}) {
  const params = new URLSearchParams({
    q: query,
    category: category || '',
    path: path || '/',
    limit: limit.toString(),
    offset: offset.toString()
  })
  const res = await fetch(`${API_BASE}/api/search?${params.toString()}`, {
    signal: options.signal
  })
  if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`)
  return await res.json()
}

export async function fetchInfo() {
  const res = await fetch(`${API_BASE}/api/info`)
  if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`)
  return await res.json()
}

export async function uploadFiles(path, files, onProgress, onAbortRef) {
  const formData = new FormData()
  for (let i = 0; i < files.length; i++) {
    formData.append('files', files[i], files[i].name)
  }

  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', `${API_BASE}/api/upload?path=${encodeURIComponent(path)}`)

    if (typeof onAbortRef === 'function') {
      onAbortRef(() => xhr.abort())
    }

    if (xhr.upload && onProgress) {
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) {
          const percent = Math.round((e.loaded / e.total) * 100)
          onProgress(percent, e.loaded, e.total)
        }
      }
    }

    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          resolve(JSON.parse(xhr.responseText))
        } catch {
          resolve({ success: true })
        }
      } else {
        reject(new Error((xhr.responseText || '').trim() || `Upload failed: ${xhr.statusText}`))
      }
    }

    xhr.onabort = () => reject(new Error('Upload cancelled'))
    xhr.onerror = () => reject(new Error('Network error during upload'))
    xhr.send(formData)
  })
}

export async function createFolder(path, name) {
  const res = await fetch(`${API_BASE}/api/mkdir`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path, name })
  })
  if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`)
  return await res.json()
}

export async function deleteItem(path) {
  const res = await fetch(`${API_BASE}/api/delete`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path })
  })
  if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`)
  return await res.json()
}

export async function inspectArchive(path, options = {}) {
  const res = await fetch(`${API_BASE}/api/archive?path=${encodeURIComponent(path)}`, {
    signal: options.signal
  })
  if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`)
  return await res.json()
}

export async function fetchShares() {
  const res = await fetch(`${API_BASE}/api/shares`)
  if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`)
  return await res.json()
}

export async function createShareLink(payload) {
  const res = await fetch(`${API_BASE}/api/shares`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload)
  })
  if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`)
  return await res.json()
}

export async function deleteShareLink(id) {
  const res = await fetch(`${API_BASE}/api/shares?id=${encodeURIComponent(id)}`, {
    method: 'DELETE'
  })
  if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`)
  return await res.json()
}

export async function updateShareConfig(payload) {
  const res = await fetch(`${API_BASE}/api/shares/config`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload)
  })
  if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`)
  return await res.json()
}
