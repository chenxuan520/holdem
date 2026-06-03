import { useState } from 'react'
import { probeInlinePreset } from '../lib/api'
import { extractInlineConfig, makeCustomPresetId } from '../lib/customPresets'
import type { CustomPresetEntry, StructuredOutputMode } from '../lib/types'

type Props = {
  // When editing an existing entry, this is the current value; null means
  // "creating a new one".
  initial: CustomPresetEntry | null
  onSubmit: (entry: CustomPresetEntry) => void
  onCancel: () => void
  onDelete?: (id: string) => void
}

const STRUCTURED_MODES: { value: StructuredOutputMode; label: string; hint: string }[] = [
  { value: 'tool_call', label: 'tool_call', hint: 'OpenAI tools 调用，最稳定（GPT 类）' },
  { value: 'json_object', label: 'json_object', hint: 'response_format 强制 JSON（GLM/Kimi/DeepSeek）' },
  { value: 'none', label: 'none', hint: '只靠 prompt 自然语言约束 + parser fallback' },
]

export function CustomPresetForm({ initial, onSubmit, onCancel, onDelete }: Props) {
  const [name, setName] = useState(initial?.name ?? '')
  const [endpoint, setEndpoint] = useState(initial?.endpoint ?? '')
  const [token, setToken] = useState(initial?.token ?? '')
  const [model, setModel] = useState(initial?.model ?? '')
  const [structuredOutput, setStructuredOutput] = useState<StructuredOutputMode>(
    initial?.structuredOutput ?? 'tool_call',
  )
  const [systemPrompt, setSystemPrompt] = useState(initial?.systemPrompt ?? '')
  // Advanced (optional) knobs. Start expanded only when the entry already uses
  // them, so the common case stays uncluttered.
  const [maxTokens, setMaxTokens] = useState(initial?.maxTokens ? String(initial.maxTokens) : '')
  const [extraBodyText, setExtraBodyText] = useState(
    initial?.extraBody ? JSON.stringify(initial.extraBody, null, 2) : '',
  )
  const [advancedOpen, setAdvancedOpen] = useState(Boolean(initial?.maxTokens || initial?.extraBody))
  const [probing, setProbing] = useState(false)
  const [probeResult, setProbeResult] = useState<{ ok: boolean; message: string } | null>(null)
  const [error, setError] = useState<string | null>(null)

  const trimmedName = name.trim()
  const trimmedEndpoint = endpoint.trim()
  const trimmedToken = token.trim()
  const trimmedModel = model.trim()
  const canSubmit = trimmedName !== '' && trimmedEndpoint !== '' && trimmedToken !== '' && trimmedModel !== ''

  // buildEntry validates the advanced fields and assembles the entry, or
  // returns a human-readable error. Shared by submit and the connectivity
  // probe so the two paths never disagree about what counts as valid.
  function buildEntry(): { entry: CustomPresetEntry } | { error: string } {
    if (!canSubmit) {
      return { error: '请补全名称 / endpoint / token / model 四个必填字段。' }
    }
    let maxTokensValue: number | undefined
    const mt = maxTokens.trim()
    if (mt !== '') {
      const n = Number(mt)
      if (!Number.isInteger(n) || n <= 0) {
        return { error: 'max_tokens 必须是正整数（留空则用后端默认）。' }
      }
      maxTokensValue = n
    }
    let extraBodyValue: Record<string, unknown> | undefined
    const eb = extraBodyText.trim()
    if (eb !== '') {
      let parsed: unknown
      try {
        parsed = JSON.parse(eb)
      } catch {
        return { error: 'extra_body 不是合法 JSON。' }
      }
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        return { error: 'extra_body 必须是一个 JSON 对象，例如 {"thinking":{"type":"disabled"}}。' }
      }
      extraBodyValue = parsed as Record<string, unknown>
    }
    return {
      entry: {
        id: initial?.id ?? makeCustomPresetId(),
        name: trimmedName,
        endpoint: trimmedEndpoint,
        token: trimmedToken,
        model: trimmedModel,
        structuredOutput,
        systemPrompt: systemPrompt.trim(),
        maxTokens: maxTokensValue,
        extraBody: extraBodyValue,
      },
    }
  }

  return (
    <form
      className="custom-preset-form"
      onSubmit={(event) => {
        event.preventDefault()
        const result = buildEntry()
        if ('error' in result) {
          setError(result.error)
          return
        }
        setError(null)
        onSubmit(result.entry)
      }}
    >
      <div className="custom-preset-form-head">
        <strong>{initial ? '编辑自定义模型' : '添加自定义模型'}</strong>
        <p className="muted-text">
          token 只保存在你这台浏览器的 localStorage，不会进 git，也不会写入后端 SQLite；只在「检测」和「开始比赛」时
          以 inline 方式发到后端。
        </p>
      </div>

      <div className="custom-preset-form-grid">
        <label>
          <span>显示名称</span>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="例如：我的 Claude Sonnet"
            autoComplete="off"
          />
        </label>
        <label>
          <span>endpoint</span>
          <input
            type="text"
            value={endpoint}
            onChange={(e) => setEndpoint(e.target.value)}
            placeholder="https://api.example.com/v1"
            autoComplete="off"
            spellCheck={false}
          />
        </label>
        <label>
          <span>token</span>
          <input
            type="password"
            value={token}
            onChange={(e) => setToken(e.target.value)}
            placeholder="sk-... / at-..."
            autoComplete="off"
            spellCheck={false}
          />
        </label>
        <label>
          <span>model</span>
          <input
            type="text"
            value={model}
            onChange={(e) => setModel(e.target.value)}
            placeholder="例如：gpt-5.4 / glm-5 / claude-sonnet-4"
            autoComplete="off"
            spellCheck={false}
          />
        </label>
        <label>
          <span>结构化输出</span>
          <select
            value={structuredOutput}
            onChange={(e) => setStructuredOutput(e.target.value as StructuredOutputMode)}
          >
            {STRUCTURED_MODES.map((mode) => (
              <option key={mode.value} value={mode.value}>
                {mode.label} — {mode.hint}
              </option>
            ))}
          </select>
        </label>
        <label className="custom-preset-form-wide">
          <span>system prompt（可选；建议留空保持 benchmark 公平）</span>
          <textarea
            value={systemPrompt}
            onChange={(e) => setSystemPrompt(e.target.value)}
            rows={3}
            placeholder="留空即使用后端默认决策框架，所有模型在同一公平起点上 PK。"
          />
        </label>
      </div>

      <div className="custom-preset-form-advanced">
        <button type="button" className="ghost-button compact" onClick={() => setAdvancedOpen((v) => !v)}>
          {advancedOpen ? '收起高级（可选）' : '展开高级（可选）'}
        </button>
        {advancedOpen ? (
          <div className="custom-preset-form-grid">
            <label>
              <span>max_tokens（可选）</span>
              <input
                type="number"
                min={1}
                value={maxTokens}
                onChange={(e) => setMaxTokens(e.target.value)}
                placeholder="推理模型建议 4096；留空用默认"
              />
            </label>
            <label className="custom-preset-form-wide">
              <span>extra_body（可选，JSON 对象，原样并入请求体）</span>
              <textarea
                value={extraBodyText}
                onChange={(e) => setExtraBodyText(e.target.value)}
                rows={4}
                spellCheck={false}
                placeholder={'厂商私有参数。例如关掉 DeepSeek 思考：\n{"thinking": {"type": "disabled"}}'}
              />
            </label>
          </div>
        ) : null}
      </div>

      {error ? <p className="error-text">{error}</p> : null}
      {probeResult ? (
        <p className={`probe-summary ${probeResult.ok ? 'ok' : 'error'}`}>{probeResult.message}</p>
      ) : null}

      <div className="custom-preset-form-actions">
        <button
          type="button"
          className="ghost-button"
          disabled={!canSubmit || probing}
          onClick={async () => {
            const result = buildEntry()
            if ('error' in result) {
              setError(result.error)
              return
            }
            setError(null)
            setProbing(true)
            setProbeResult(null)
            try {
              const r = await probeInlinePreset(extractInlineConfig(result.entry))
              if (r.ok) {
                setProbeResult({
                  ok: true,
                  message: `连通正常 · ${r.latencyMs}ms${r.model ? ` · ${r.model}` : ''}`,
                })
              } else {
                setProbeResult({ ok: false, message: `连通失败：${r.error || '未知错误'}` })
              }
            } catch (err) {
              setProbeResult({
                ok: false,
                message: `连通失败：${err instanceof Error ? err.message : String(err)}`,
              })
            } finally {
              setProbing(false)
            }
          }}
        >
          {probing ? '检测中...' : '检测连通性'}
        </button>

        <div className="custom-preset-form-actions-end">
          {initial && onDelete ? (
            <button
              type="button"
              className="danger-button"
              onClick={() => {
                if (!window.confirm(`确定删除自定义模型 "${initial.name}" 吗？`)) return
                onDelete(initial.id)
              }}
            >
              删除
            </button>
          ) : null}
          <button type="button" className="ghost-button" onClick={onCancel}>
            取消
          </button>
          <button type="submit" className="primary-button" disabled={!canSubmit}>
            {initial ? '保存修改' : '添加'}
          </button>
        </div>
      </div>
    </form>
  )
}
