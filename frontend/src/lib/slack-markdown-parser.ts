export const enum NodeType {
  Text,
  Bold,
  Italic,
  Strike,
  Code,
  PreText,
  Link,
}

export type Node =
  | { type: NodeType.Text; text: string }
  | { type: NodeType.Bold; children: Node[] }
  | { type: NodeType.Italic; children: Node[] }
  | { type: NodeType.Strike; children: Node[] }
  | { type: NodeType.Code; text: string }
  | { type: NodeType.PreText; text: string }
  | { type: NodeType.Link; url: string; label?: Node[] };

type Parser = (
  text: string,
  pos: number,
  parseText: (t: string) => Node[],
) => [Node, number] | null;

function isExplicitBoundary(ch: string): boolean {
  return !ch || /[\s.,([{!?\-=\])}:;'">/]/.test(ch);
}

const parsePreText: Parser = (text, pos) => {
  if (!isExplicitBoundary(text.charAt(pos - 1))) return null;
  const rest = text.substring(pos);
  const m = rest.match(/^```(\s*\S[\s\S]*?\s*)```(?=[\s.,\])}!?\-=]|$)/);
  if (!m) return null;
  return [{ type: NodeType.PreText, text: m[1] }, pos + m[0].length];
};

const parseCode: Parser = (text, pos) => {
  if (!isExplicitBoundary(text.charAt(pos - 1))) return null;
  const rest = text.substring(pos);
  const m = rest.match(/^`([^`\n]+?)`(?=[\s.,\])}!?\-=]|$)/);
  if (!m) return null;
  return [{ type: NodeType.Code, text: m[1] }, pos + m[0].length];
};

function makeWrappingParser(
  nodeType: NodeType.Bold | NodeType.Italic | NodeType.Strike,
  marker: string,
): Parser {
  const escaped = marker.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const pattern = new RegExp(
    `^${escaped}(\\S([^${escaped}\\n]*?\\S)?)${escaped}(?=[\\s.,\\])}!?\\-=:;'">]|$)`,
  );

  return (text, pos, parseText) => {
    if (!isExplicitBoundary(text.charAt(pos - 1))) return null;
    const rest = text.substring(pos);
    const m = rest.match(pattern);
    if (!m) return null;
    return [
      { type: nodeType, children: parseText(m[1]) } as Node,
      pos + m[0].length,
    ];
  };
}

const parseBold = makeWrappingParser(NodeType.Bold, "*");
const parseItalic = makeWrappingParser(NodeType.Italic, "_");
const parseStrike = makeWrappingParser(NodeType.Strike, "~");

const parseLink: Parser = (text, pos, parseText) => {
  const rest = text.substring(pos);
  const m = rest.match(/^<([^\s<>][^\n<>]*?)(\|([^<>]+?))?>/)
  if (!m) return null;
  const [matched, rawUrl, , label] = m;
  if (rawUrl.startsWith("@") || rawUrl.startsWith("#") || rawUrl.startsWith("!")) {
    return null;
  }
  if (!/^https?:\/\//.test(rawUrl) && !rawUrl.startsWith("mailto:")) {
    return null;
  }
  return [
    {
      type: NodeType.Link,
      url: rawUrl,
      label: label ? parseText(label) : undefined,
    },
    pos + matched.length,
  ];
};

const parsers: Parser[] = [
  parsePreText,
  parseCode,
  parseBold,
  parseItalic,
  parseStrike,
  parseLink,
];

export function parseSlackMarkdown(text: string): Node[] {
  const children: Node[] = [];
  let buffer = "";

  const flush = () => {
    if (buffer) {
      children.push({ type: NodeType.Text, text: buffer });
      buffer = "";
    }
  };

  let i = 0;
  while (i < text.length) {
    let matched = false;
    for (const parser of parsers) {
      const result = parser(text, i, parseSlackMarkdown);
      if (result) {
        flush();
        children.push(result[0]);
        i = result[1];
        matched = true;
        break;
      }
    }
    if (!matched) {
      buffer += text.charAt(i);
      i++;
    }
  }

  flush();
  return children;
}
