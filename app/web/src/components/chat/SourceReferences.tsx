import { safeSourceUrl } from '../../chat/types'
import type { SourceReference } from '../../chat/types'

export function SourceReferences({ sources }: { sources: SourceReference[] }) {
  const safe = sources.flatMap(source => { const url = safeSourceUrl(source.url); return url ? [{ ...source, url }] : [] })
  if (!safe.length) return null
  return <details className="chat-sources" open><summary>Source references <small>Synthetic samples, not evidence</small></summary>
    <ul>{safe.map((source, index) => <li key={`${source.url}-${index}`}><a href={source.url} target="_blank" rel="noopener noreferrer">{source.title}</a></li>)}</ul>
  </details>
}
