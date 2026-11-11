import { LoaderIcon } from "lucide-react";

function Loading() {
  return (
    <div className="min-h-[100svh] w-full flex items-center justify-center bg-gradient-to-br from-stone-50 via-white to-slate-100 px-6 py-10 dark:from-zinc-950 dark:via-zinc-900 dark:to-zinc-950">
      <div className="flex flex-col items-center gap-4 rounded-3xl border border-stone-200 bg-white/90 px-8 py-10 shadow-[0_20px_70px_rgba(15,23,42,0.08)] backdrop-blur dark:border-zinc-800 dark:bg-zinc-900/90">
        <LoaderIcon className="h-8 w-8 animate-spin text-zinc-500 dark:text-zinc-300" />
        <div className="text-sm font-medium text-zinc-500 dark:text-zinc-400">正在加载应用内容</div>
      </div>
    </div>
  );
}

export default Loading;
