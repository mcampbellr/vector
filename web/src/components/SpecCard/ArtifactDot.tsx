import styles from './SpecCard.module.css'

interface ArtifactDotProps {
  label: string
  on: boolean
}

// Tiny indicator for an OpenSpec artifact (proposal/design/tasks) on a card.
export function ArtifactDot({ label, on }: ArtifactDotProps) {
  return <span className={`${styles.artifact} ${on ? styles.artifactOn : ''}`}>{label}</span>
}
