import type { ComponentType } from "react";

const CHUNK_LOAD_RETRY_KEY = "memos:chunk-load-retried";

const isChunkLoadError = (error: unknown) => {
  const message = error instanceof Error ? error.message : String(error ?? "");
  return [
    "Failed to fetch dynamically imported module",
    "Importing a module script failed",
    "error loading dynamically imported module",
    "ChunkLoadError",
  ].some((pattern) => message.includes(pattern));
};

const reloadForChunkLoad = () => {
  if (typeof window === "undefined") {
    return;
  }

  const hasRetried = window.sessionStorage.getItem(CHUNK_LOAD_RETRY_KEY) === "1";
  if (hasRetried) {
    return;
  }

  window.sessionStorage.setItem(CHUNK_LOAD_RETRY_KEY, "1");
  window.location.reload();
};

export const clearChunkLoadRetryFlag = () => {
  if (typeof window === "undefined") {
    return;
  }

  window.sessionStorage.removeItem(CHUNK_LOAD_RETRY_KEY);
};

export const lazyWithRetry = <T extends { default: ComponentType<any> }>(importer: () => Promise<T>) => {
  return async () => {
    try {
      const module = await importer();
      clearChunkLoadRetryFlag();
      return module;
    } catch (error) {
      if (isChunkLoadError(error)) {
        reloadForChunkLoad();
      }
      throw error;
    }
  };
};
