import { type Node, NodeType, parseSlackMarkdown } from "../lib/slack-markdown-parser";
import { cn } from "../lib/utils";

function renderNode(node: Node, key: number): React.ReactNode {
  switch (node.type) {
    case NodeType.Text:
      return <span key={key}>{node.text}</span>;
    case NodeType.Bold:
      return (
        <strong key={key} className="font-semibold">
          {node.children.map((c, i) => renderNode(c, i))}
        </strong>
      );
    case NodeType.Italic:
      return (
        <em key={key}>
          {node.children.map((c, i) => renderNode(c, i))}
        </em>
      );
    case NodeType.Strike:
      return (
        <del key={key}>
          {node.children.map((c, i) => renderNode(c, i))}
        </del>
      );
    case NodeType.Code:
      return (
        <code
          key={key}
          className="font-mono text-[12px] px-1.5 py-px bg-bg-sunken text-ink-2 rounded-1 border border-line"
        >
          {node.text}
        </code>
      );
    case NodeType.PreText:
      return (
        <pre
          key={key}
          className="font-mono text-[12px] p-3 my-1.5 bg-bg-sunken rounded-2 border border-line overflow-x-auto"
        >
          <code>{node.text}</code>
        </pre>
      );
    case NodeType.Link:
      return (
        <a
          key={key}
          href={node.url}
          target="_blank"
          rel="noopener noreferrer"
          className="text-info border-b border-info/30 hover:border-info"
        >
          {node.label
            ? node.label.map((c, i) => renderNode(c, i))
            : node.url}
        </a>
      );
  }
}

interface SlackMarkdownProps {
  text: string;
  className?: string;
}

export function SlackMarkdown({ text, className }: SlackMarkdownProps) {
  const nodes = parseSlackMarkdown(text);
  return (
    <div
      className={cn(
        "text-[13.5px] leading-[1.55] text-ink-1 whitespace-pre-wrap",
        className,
      )}
    >
      {nodes.map((node, i) => renderNode(node, i))}
    </div>
  );
}
