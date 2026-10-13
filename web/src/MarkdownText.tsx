import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";

const remarkPlugins = [remarkGfm];

export function MarkdownText({ text }: { text: string }) {
  return <div className="markdown"><Markdown remarkPlugins={remarkPlugins}>{text}</Markdown></div>;
}
