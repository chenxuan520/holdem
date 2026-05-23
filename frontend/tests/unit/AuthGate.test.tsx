import React from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AuthGate } from '../../src/components/AuthGate'

// Test-time localStorage shim. The vitest+jsdom combo on this machine
// ships a partial localStorage that throws on getItem/setItem, so the
// AuthGate's silent-error fallbacks would mask test setup. We replace
// the storage with an in-memory map for the duration of the suite so
// password-persistence assertions are observable.
class InMemoryStorage {
  private map = new Map<string, string>()
  get length() {
    return this.map.size
  }
  clear() {
    this.map.clear()
  }
  getItem(key: string): string | null {
    return this.map.has(key) ? this.map.get(key)! : null
  }
  key(idx: number): string | null {
    return Array.from(this.map.keys())[idx] ?? null
  }
  removeItem(key: string): void {
    this.map.delete(key)
  }
  setItem(key: string, value: string): void {
    this.map.set(key, String(value))
  }
}

const STORAGE_KEY = 'holdem.password'

describe('AuthGate', () => {
  const fetchSpy = vi.spyOn(globalThis, 'fetch')
  const memoryStorage = new InMemoryStorage()

  beforeEach(() => {
    fetchSpy.mockReset()
    memoryStorage.clear()
    Object.defineProperty(window, 'localStorage', {
      configurable: true,
      value: memoryStorage,
    })
  })

  afterEach(() => {
    fetchSpy.mockReset()
  })

  it('renders children directly when backend reports auth disabled', async () => {
    fetchSpy.mockResolvedValueOnce(
      new Response(JSON.stringify({ authRequired: false, ok: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    render(
      <AuthGate>
        <div>secret-app</div>
      </AuthGate>,
    )

    await waitFor(() => {
      expect(screen.getByText('secret-app')).toBeInTheDocument()
    })
  })

  it('shows the login form when backend requires auth and nothing is stored', async () => {
    fetchSpy.mockResolvedValueOnce(
      new Response(JSON.stringify({ authRequired: true, ok: false, error: 'auth required' }), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    render(
      <AuthGate>
        <div>secret-app</div>
      </AuthGate>,
    )

    await waitFor(() => {
      expect(screen.getByText('需要密码进入')).toBeInTheDocument()
    })
    expect(screen.queryByText('secret-app')).toBeNull()
  })

  it('accepts a correct password and reveals children', async () => {
    const user = userEvent.setup()

    // Initial check (no stored password) → 401.
    fetchSpy.mockResolvedValueOnce(
      new Response(JSON.stringify({ authRequired: true, ok: false }), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    // Submit check (with typed password) → 200.
    fetchSpy.mockResolvedValueOnce(
      new Response(JSON.stringify({ authRequired: true, ok: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    render(
      <AuthGate>
        <div>secret-app</div>
      </AuthGate>,
    )

    const input = await screen.findByLabelText('访问密码')
    await user.type(input, 'letmein')
    await user.click(screen.getByRole('button', { name: '进入' }))

    await waitFor(() => {
      expect(screen.getByText('secret-app')).toBeInTheDocument()
    })
    // Password persisted so reload doesn't re-prompt.
    expect(window.localStorage.getItem(STORAGE_KEY)).toBe('letmein')
  })

  it('reports an error when the typed password is wrong and keeps the form open', async () => {
    const user = userEvent.setup()

    fetchSpy.mockResolvedValue(
      new Response(JSON.stringify({ authRequired: true, ok: false, error: 'auth required' }), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    render(
      <AuthGate>
        <div>secret-app</div>
      </AuthGate>,
    )

    const input = await screen.findByLabelText('访问密码')
    await user.type(input, 'wrong-pw')
    await user.click(screen.getByRole('button', { name: '进入' }))

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument()
    })
    expect(screen.queryByText('secret-app')).toBeNull()
    expect(window.localStorage.getItem(STORAGE_KEY)).toBeNull()
  })

  it('returns to the login form when a holdem:auth-required event fires later', async () => {
    fetchSpy.mockResolvedValueOnce(
      new Response(JSON.stringify({ authRequired: false, ok: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    render(
      <AuthGate>
        <div>secret-app</div>
      </AuthGate>,
    )

    await waitFor(() => {
      expect(screen.getByText('secret-app')).toBeInTheDocument()
    })

    window.dispatchEvent(new CustomEvent('holdem:auth-required'))

    await waitFor(() => {
      expect(screen.getByText('需要密码进入')).toBeInTheDocument()
    })
  })
})
