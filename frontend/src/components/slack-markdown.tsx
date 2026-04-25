import { type Node, NodeType, parseSlackMarkdown } from "../lib/slack-markdown-parser";

function renderNode(node: Node, key: number): React.ReactNode {
  switch (node.type) {
    case NodeType.Text:
      return <span key={key}>{node.text}</span>;
    case NodeType.Bold:
      return (
        <strong key={key}>
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
          className="bg-gray-100 text-red-600 px-1 py-0.5 rounded text-xs font-mono"
        >
          {node.text}
        </code>
      );
    case NodeType.PreText:
      return (
        <pre
          key={key}
          className="bg-gray-100 p-3 rounded text-sm font-mono overflow-x-auto my-1"
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
          className="text-blue-600 hover:underline"
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
    <span className={className}>
      {nodes.map((node, i) => renderNode(node, i))}
    </span>
  );
}
