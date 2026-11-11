import { LoaderIcon } from "lucide-react";
import { Suspense, lazy } from "react";
import type { Props } from "./MemoEditor";

const MemoEditor = lazy(() => import("./MemoEditor"));

const MemoEditorFallback = ({ className }: { className?: string }) => (
  <div className={className ? `w-full ${className}` : "w-full py-4"}>
    <div className="w-full flex flex-row justify-center items-center py-4">
      <LoaderIcon className="animate-spin text-zinc-500" />
    </div>
  </div>
);

const MemoEditorLazy = (props: Props) => {
  return (
    <Suspense fallback={<MemoEditorFallback className={props.className} />}>
      <MemoEditor {...props} />
    </Suspense>
  );
};

export default MemoEditorLazy;
