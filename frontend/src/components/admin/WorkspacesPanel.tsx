import { useEffect, useState } from 'react'
import { createWorkspace, deleteWorkspace, listWorkspaces } from '../../api/admin'
import type { Workspace } from '../../types/auth'

export function WorkspacesPanel({ token }: { token: string }) {
  const [items, setItems] = useState<Workspace[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [newName, setNewName] = useState('')
  const [newDesc, setNewDesc] = useState('')

  async function reload() {
    setLoading(true)
    try {
      setItems(await listWorkspaces(token))
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    reload()
  }, [token])

  async function onCreate(e: React.FormEvent) {
    e.preventDefault()
    if (!newName.trim()) return
    try {
      await createWorkspace(token, newName.trim(), newDesc.trim() || undefined)
      setNewName('')
      setNewDesc('')
      reload()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  async function onDelete(id: string, name: string) {
    if (!confirm(`"${name}" workspace'ini silmek istediğinden emin misin?`)) return
    try {
      await deleteWorkspace(token, id)
      reload()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <h2 style={{ marginTop: 0 }}>Workspace'ler</h2>

      <form onSubmit={onCreate} style={{ display: 'flex', gap: 8, alignItems: 'flex-end' }}>
        <label style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          <span style={{ fontSize: 12, color: '#6b7280' }}>İsim</span>
          <input value={newName} onChange={(e) => setNewName(e.target.value)} style={inputStyle} required />
        </label>
        <label style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          <span style={{ fontSize: 12, color: '#6b7280' }}>Açıklama</span>
          <input value={newDesc} onChange={(e) => setNewDesc(e.target.value)} style={inputStyle} />
        </label>
        <button type="submit" style={{ padding: '8px 14px', background: '#4f46e5', color: 'white', border: 0, borderRadius: 4, cursor: 'pointer' }}>
          Oluştur
        </button>
      </form>

      {loading && <div>Yükleniyor…</div>}
      {error && <div style={{ color: 'crimson' }}>Hata: {error}</div>}

      <ul style={{ listStyle: 'none', padding: 0, display: 'flex', flexDirection: 'column', gap: 6 }}>
        {items.map((w) => (
          <li key={w.id} style={{ padding: 10, border: '1px solid var(--border-color, #e5e7eb)', borderRadius: 6, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <div>
              <strong>{w.name}</strong>
              {w.is_personal && <span style={{ marginLeft: 8, fontSize: 11, color: '#6b7280' }}>(kişisel)</span>}
              {w.description && <div style={{ fontSize: 12, color: '#6b7280' }}>{w.description}</div>}
              <div style={{ fontSize: 11, color: '#9ca3af', fontFamily: 'monospace' }}>{w.slug}</div>
            </div>
            {!w.is_personal && (
              <button onClick={() => onDelete(w.id, w.name)} style={btnSecondary}>Sil</button>
            )}
          </li>
        ))}
      </ul>
    </div>
  )
}

const inputStyle: React.CSSProperties = { padding: 8, border: '1px solid #d1d5db', borderRadius: 4, minWidth: 200 }
const btnSecondary: React.CSSProperties = { padding: '4px 10px', background: 'transparent', border: '1px solid #d1d5db', borderRadius: 4, cursor: 'pointer' }
