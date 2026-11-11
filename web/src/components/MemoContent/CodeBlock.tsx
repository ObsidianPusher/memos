import copy from "copy-to-clipboard";
import { CopyIcon } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import toast from "react-hot-toast";
import { cn } from "@/utils";
import MermaidBlock from "./MermaidBlock";
import { BaseProps } from "./types";

// Special languages that are rendered differently.
enum SpecialLanguage {
  HTML = "__html",
  MERMAID = "mermaid",
}

interface Props extends BaseProps {
  language: string;
  content: string;
}

let highlightJsModulePromise: Promise<typeof import("highlight.js")> | null = null;

const loadHighlightJs = () => {
  if (!highlightJsModulePromise) {
    highlightJsModulePromise = import("highlight.js");
  }
  return highlightJsModulePromise;
};

const escapeHtml = (value: string) =>
  value.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&#39;");

const CodeBlock: React.FC<Props> = ({ language, content }: Props) => {
  const formatedLanguage = useMemo(() => (language || "").toLowerCase() || "text", [language]);
  const isHtml = formatedLanguage === SpecialLanguage.HTML;
  const isMermaid = formatedLanguage === SpecialLanguage.MERMAID;
  const [highlightedCode, setHighlightedCode] = useState(() => escapeHtml(content));

  useEffect(() => {
    if (isHtml || isMermaid) {
      return;
    }

    let cancelled = false;
    setHighlightedCode(escapeHtml(content));

    void loadHighlightJs().then(({ default: hljs }) => {
      if (cancelled) {
        return;
      }

      try {
        if (hljs.getLanguage(formatedLanguage)) {
          setHighlightedCode(
            hljs.highlight(content, {
              language: formatedLanguage,
            }).value,
          );
        }
      } catch {
        setHighlightedCode(escapeHtml(content));
      }
    });

    return () => {
      cancelled = true;
    };
  }, [content, formatedLanguage, isHtml, isMermaid]);

  // Users can set Markdown code blocks as `__html` to render HTML directly.
  if (isHtml) {
    return (
      <div
        className="w-full overflow-auto !my-2"
        dangerouslySetInnerHTML={{
          __html: content,
        }}
      />
    );
  } else if (isMermaid) {
    return <MermaidBlock content={content} />;
  }

  const handleCopyButtonClick = useCallback(() => {
    copy(content);
    toast.success("Copied to clipboard!");
  }, [content]);

  return (
    <div className="w-full my-1 bg-amber-100 border-l-4 border-amber-400 rounded hover:shadow dark:bg-zinc-600 dark:border-zinc-400 relative">
      <div className="w-full px-2 py-1 flex flex-row justify-between items-center text-amber-500 dark:text-zinc-400">
        <span className="text-sm font-mono">{formatedLanguage}</span>
        <CopyIcon className="w-4 h-auto cursor-pointer hover:opacity-80" onClick={handleCopyButtonClick} />
      </div>

      <div className="overflow-auto">
        <pre className={cn("no-wrap overflow-auto", "w-full p-2 bg-amber-50 dark:bg-zinc-700 relative")}>
          <code
            className={cn(`language-${formatedLanguage}`, "block text-sm leading-5")}
            dangerouslySetInnerHTML={{ __html: highlightedCode }}
          ></code>
        </pre>
      </div>
    </div>
  );
};

export default CodeBlock;
