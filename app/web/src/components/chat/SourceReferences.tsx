import { safeSourceUrl } from '../../chat/types'
import type { SourceReference } from '../../chat/types'

export function SourceReferences({ sources }: { sources: SourceReference[] }) {
  if (!sources.length) return null
  return (
    <details className="chat-sources" open>
      <summary>Source references</summary>
      <ul>
        {sources.map(source => {

          // Document/page references may have no public URL; do not manufacture a link.

          const url = source.url && safeSourceUrl(source.url)
          const label = `${source.title}${source.page != null ? `, p. ${source.page}` : ''}`
          return (
            <li key={source.number}>
              [{source.number}] {url ? (
                <a href={url} target="_blank" rel="noopener noreferrer">{label}</a>
              ) : label}
            </li>
          )
        })}
      </ul>
    </details>
  )
}
