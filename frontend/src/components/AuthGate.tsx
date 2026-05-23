import { useCallback, useEffect, useState } from 'react'
import { checkAuth, clearStoredPassword, getStoredPassword, setStoredPassword } from '../lib/auth'

type GateState =
  | { kind: 'checking' }
  | { kind: 'open' } // backend has no password configured, or ours verified
  | { kind: 'locked'; error?: string }

type Props = {
  children: React.ReactNode
}

export function AuthGate({ children }: Props) {
  const [state, setState] = useState<GateState>({ kind: 'checking' })
  const [pendingPassword, setPendingPassword] = useState<string>('')
  const [submitting, setSubmitting] = useState(false)

  const verify = useCallback(async (passwordOverride?: string) => {
    const result = await checkAuth(passwordOverride)
    if (!result.authRequired) {
      setState({ kind: 'open' })
      return
    }
    if (result.ok) {
      setState({ kind: 'open' })
      return
    }
    clearStoredPassword()
    setState({ kind: 'locked', error: result.error })
  }, [])

  useEffect(() => {
    void verify(getStoredPassword() || undefined)
  }, [verify])

  // If any later API call hits 401, lib/api.ts dispatches this event. We
  // immediately swap the UI back to the login form so the user gets a
  // clear retry instead of a silent failure.
  useEffect(() => {
    if (typeof window === 'undefined') return
    const onAuthRequired = () => {
      setState({ kind: 'locked', error: '密码失效或已被更改，请重新登录。' })
    }
    window.addEventListener('holdem:auth-required', onAuthRequired)
    return () => window.removeEventListener('holdem:auth-required', onAuthRequired)
  }, [])

  if (state.kind === 'checking') {
    return (
      <div className="auth-gate-screen">
        <div className="auth-gate-card card panel">
          <p className="muted-text">正在验证登录状态...</p>
        </div>
      </div>
    )
  }

  if (state.kind === 'locked') {
    return (
      <div className="auth-gate-screen">
        <form
          className="auth-gate-card card panel"
          onSubmit={async (event) => {
            event.preventDefault()
            const password = pendingPassword.trim()
            if (!password) {
              setState({ kind: 'locked', error: '请输入密码' })
              return
            }
            setSubmitting(true)
            try {
              const result = await checkAuth(password)
              if (result.ok) {
                setStoredPassword(password)
                setPendingPassword('')
                setState({ kind: 'open' })
              } else {
                setState({ kind: 'locked', error: result.error || '密码错误' })
              }
            } finally {
              setSubmitting(false)
            }
          }}
        >
          <header className="auth-gate-header">
            <strong>需要密码进入</strong>
            <p className="muted-text">这个 Holdem 实例启用了访问控制。请输入操作员设置的密码。</p>
          </header>

          <label className="auth-gate-field">
            <span>访问密码</span>
            <input
              type="password"
              autoFocus
              value={pendingPassword}
              onChange={(event) => setPendingPassword(event.target.value)}
              disabled={submitting}
              autoComplete="current-password"
              spellCheck={false}
            />
          </label>

          {state.error ? <p className="auth-gate-error" role="alert">{state.error}</p> : null}

          <button
            type="submit"
            className="primary-button"
            disabled={submitting || !pendingPassword.trim()}
          >
            {submitting ? '验证中...' : '进入'}
          </button>

          <p className="muted-text auth-gate-footnote">
            密码只保存在浏览器 localStorage，刷新后仍然有效；如果你换了一台机器需要重新输入。
          </p>
        </form>
      </div>
    )
  }

  return <>{children}</>
}
