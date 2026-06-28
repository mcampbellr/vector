import { useState } from 'react'
import { FileText } from 'lucide-react'
import type { Card } from '../../types/board'
import { type ArtifactEntry, entriesFor } from './entries'
import { FilePreviewModal } from './FilePreviewModal'
import styles from './SpecDetailsDrawer.module.css'

interface SpecArtifactBrowserProps {
  card: Card
}

// SpecArtifactBrowser lists a spec's source documents and opens a FilePreviewModal
// for the selected one. The board stays read-only — this only reads files.
export function SpecArtifactBrowser({ card }: SpecArtifactBrowserProps) {
  const [selected, setSelected] = useState<ArtifactEntry | null>(null)
  const entries = entriesFor(card)

  if (entries.length === 0) {
    return <p className={styles.muted}>No source files available.</p>
  }

  return (
    <>
      <ul className={styles.fileList}>
        {entries.map((entry) => (
          <li key={entry.key}>
            <button type="button" className={styles.fileItem} onClick={() => setSelected(entry)}>
              <FileText size={13} strokeWidth={2} />
              <span className={styles.fileName}>{entry.label}</span>
            </button>
          </li>
        ))}
      </ul>
      {selected && (
        <FilePreviewModal
          specId={card.id}
          artifact={selected.key}
          fileName={selected.label}
          onClose={() => setSelected(null)}
        />
      )}
    </>
  )
}
