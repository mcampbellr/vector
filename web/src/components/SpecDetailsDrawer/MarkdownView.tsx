import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

interface MarkdownViewProps {
  source: string
}

// MarkdownView is a thin react-markdown + remark-gfm wrapper. It is the ONLY
// module that imports react-markdown, so the dependency is isolated here and
// kept out of the initial board bundle: the file-preview modal loads it via
// React.lazy. No rehype-raw / no raw HTML — only GitHub-flavored Markdown is
// rendered (the source is trusted committed state, but raw HTML stays disabled
// by design).
export default function MarkdownView({ source }: MarkdownViewProps) {
  return <Markdown remarkPlugins={[remarkGfm]}>{source}</Markdown>
}
