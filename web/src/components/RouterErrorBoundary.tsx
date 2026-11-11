import { Button } from "@usememos/mui";
import { AlertTriangleIcon, HomeIcon, RefreshCwIcon } from "lucide-react";
import { useRouteError } from "react-router-dom";
import useNavigateTo from "@/hooks/useNavigateTo";
import { Routes } from "@/router/routes";
import { getErrorMessage } from "@/utils/error";

const isDynamicImportFailure = (message: string) => {
  return [
    "Failed to fetch dynamically imported module",
    "Importing a module script failed",
    "error loading dynamically imported module",
    "ChunkLoadError",
  ].some((pattern) => message.includes(pattern));
};

function RouterErrorBoundary() {
  const error = useRouteError();
  const navigateTo = useNavigateTo();
  const message = getErrorMessage(error);
  const isChunkFailure = isDynamicImportFailure(message);
  const title = isChunkFailure ? "页面资源已更新" : "页面暂时无法打开";
  const description = isChunkFailure
    ? "当前页面使用的是旧版缓存资源。刷新后会自动加载最新版本。"
    : "应用遇到了异常，但核心数据并未受影响。你可以刷新页面，或先返回首页继续使用。";

  return (
    <div className="min-h-screen w-full bg-gradient-to-br from-stone-50 via-white to-slate-100 px-6 py-10 dark:from-zinc-950 dark:via-zinc-900 dark:to-zinc-950">
      <div className="mx-auto flex min-h-[80vh] max-w-2xl items-center justify-center">
        <div className="w-full rounded-3xl border border-stone-200 bg-white p-6 sm:p-8 shadow-[0_24px_80px_rgba(15,23,42,0.08)] dark:border-zinc-800 dark:bg-zinc-900">
          <div className="mb-6 inline-flex h-14 w-14 items-center justify-center rounded-2xl bg-amber-100 text-amber-700 dark:bg-amber-500/10 dark:text-amber-300">
            <AlertTriangleIcon className="h-7 w-7" />
          </div>
          <h1 className="text-2xl sm:text-3xl font-semibold tracking-tight text-zinc-900 dark:text-zinc-100">{title}</h1>
          <p className="mt-3 text-base leading-7 text-zinc-600 dark:text-zinc-400">{description}</p>
          <div className="mt-6 rounded-2xl border border-stone-200 bg-stone-50 px-4 py-3 text-sm leading-6 text-zinc-600 break-all dark:border-zinc-800 dark:bg-zinc-950 dark:text-zinc-400">
            {message}
          </div>
          <div className="mt-8 flex flex-col sm:flex-row gap-3">
            <Button
              onClick={() => window.location.reload()}
              className="!rounded-xl !bg-emerald-600 !px-4 !py-2.5 !text-white hover:!bg-emerald-500"
            >
              <RefreshCwIcon className="mr-2 h-4 w-4" />
              刷新页面
            </Button>
            <Button variant="outlined" onClick={() => navigateTo(Routes.ROOT, { replace: true })} className="!rounded-xl !px-4 !py-2.5">
              <HomeIcon className="mr-2 h-4 w-4" />
              返回首页
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

export default RouterErrorBoundary;
