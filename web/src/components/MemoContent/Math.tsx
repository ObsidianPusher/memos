import { useEffect, useState, type ComponentType } from "react";
import { cn } from "@/utils";
import "katex/dist/katex.min.css";

interface Props {
  content: string;
  block?: boolean;
}

let katexModulePromise: Promise<typeof import("@matejmazur/react-katex")> | null = null;

const loadKatex = () => {
  if (!katexModulePromise) {
    katexModulePromise = import("@matejmazur/react-katex");
  }
  return katexModulePromise;
};

const Math: React.FC<Props> = ({ content, block }: Props) => {
  const [TeX, setTeX] = useState<null | ComponentType<any>>(null);

  useEffect(() => {
    let isMounted = true;

    void loadKatex().then((module) => {
      if (isMounted) {
        setTeX(() => module.default);
      }
    });

    return () => {
      isMounted = false;
    };
  }, []);

  if (!TeX) {
    return <span className={cn("max-w-full", block ? "block w-full" : "inline text-sm")}>{content}</span>;
  }

  return <TeX className={cn("max-w-full", block ? "w-full block" : "inline text-sm")} block={block} math={content} />;
};

export default Math;
