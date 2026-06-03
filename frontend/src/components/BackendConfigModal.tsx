import { useEffect, useState } from 'react'
import { getApiBase, setApiBase } from '../lib/apiBase'

// Shortcuts mirror the two backends this repo ships: the local Go server and
// the deployed Cloudflare Worker. Both already send permissive CORS headers,
// so the frontend can hit either directly without touching config/app.json.
const LOCAL_BACKEND = 'http://127.0.0.1:18130'
const CF_BACKEND = 'https://holdem-cf.011203.workers.dev'

type Props = {
  open: boolean
  onClose: () => void
}

export function BackendConfigModal({ open, onClose }: Props) {
  const [value, setValue] = useState('')

  // Reseed the input from storage each time the modal opens so it always
  // reflects the currently-active base.
  useEffect(() => {
    if (open) setValue(getApiBase())
  }, [open])

  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onClose])

  if (!open) return null

  const current = getApiBase()

  const handleSave = () => {
    setApiBase(value)
    // The app wires fetch + EventSource connections at startup from the base,
    // so a full reload is the simplest way to switch backends cleanly without
    // leaving stale streams pointed at the old one.
    window.location.reload()
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-card card panel" onClick={(e) => e.stopPropagation()}>
        <div className="modal-head">
          <strong>后端连接设置</strong>
          <button className="ghost-button compact" type="button" onClick={onClose}>
            关闭
          </button>
        </div>

        <p className="modal-desc">
          设置前端把 <code>/api</code> 请求直接发往哪个后端。留空表示走开发代理（vite，即{' '}
          <code>config/app.json</code> 的 <code>apiTarget</code>）。保存后会刷新页面生效。
        </p>

        <label className="modal-field">
          <span>后端地址（API base）</span>
          <input
            type="text"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            placeholder="留空 = 走开发代理"
            spellCheck={false}
            autoFocus
          />
        </label>

        <div className="modal-presets">
          <button type="button" className="ghost-button compact" onClick={() => setValue(LOCAL_BACKEND)}>
            本地后端
          </button>
          <button type="button" className="ghost-button compact" onClick={() => setValue(CF_BACKEND)}>
            线上 CF
          </button>
          <button type="button" className="ghost-button compact" onClick={() => setValue('')}>
            清空（走代理）
          </button>
        </div>

        <p className="modal-current">
          当前生效：<code>{current || '开发代理（相对 /api）'}</code>
        </p>

        <div className="modal-actions">
          <button type="button" className="ghost-button" onClick={onClose}>
            取消
          </button>
          <button type="button" className="primary-button inline" onClick={handleSave}>
            保存并刷新
          </button>
        </div>
      </div>
    </div>
  )
}
