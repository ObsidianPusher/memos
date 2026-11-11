import { Button } from "@usememos/mui";
import { useEffect, useMemo, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslate } from "@/utils/i18n";

const API_ENDPOINT = import.meta.env.VITE_API_ENDPOINT ?? window.location.origin;

interface RuntimeMemoryLimitResponse {
  enabled: boolean;
  memory_limit_bytes: number;
  memory_limit_mb: number;
  updated_ts: number;
}

const MemoryLimitSection = () => {
  const t = useTranslate();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [enabled, setEnabled] = useState(false);
  const [memoryLimitMb, setMemoryLimitMb] = useState(1024);
  const [lastUpdatedTs, setLastUpdatedTs] = useState(0);

  const hasValidInput = useMemo(() => {
    if (!enabled) {
      return true;
    }
    return Number.isFinite(memoryLimitMb) && memoryLimitMb >= 128 && memoryLimitMb <= 65536;
  }, [enabled, memoryLimitMb]);

  useEffect(() => {
    fetchMemoryLimit();
  }, []);

  const fetchMemoryLimit = async () => {
    setLoading(true);
    try {
      const response = await fetch(`${API_ENDPOINT}/api/v1/runtime/memory-limit`, {
        method: "GET",
        credentials: "include",
      });
      if (!response.ok) {
        throw new Error(await response.text());
      }
      const data = (await response.json()) as RuntimeMemoryLimitResponse;
      setEnabled(data.enabled);
      setMemoryLimitMb(data.memory_limit_mb || 1024);
      setLastUpdatedTs(data.updated_ts || 0);
    } catch {
      toast.error(t("setting.memory-limit-section.load-failed"));
    } finally {
      setLoading(false);
    }
  };

  const saveMemoryLimit = async () => {
    if (!hasValidInput) {
      toast.error(t("setting.memory-limit-section.invalid-input"));
      return;
    }
    setSaving(true);
    try {
      const response = await fetch(`${API_ENDPOINT}/api/v1/runtime/memory-limit`, {
        method: "PUT",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          enabled,
          memory_limit_mb: enabled ? memoryLimitMb : 0,
        }),
      });
      if (!response.ok) {
        throw new Error(await response.text());
      }
      const data = (await response.json()) as RuntimeMemoryLimitResponse;
      setEnabled(data.enabled);
      setMemoryLimitMb(data.memory_limit_mb || memoryLimitMb);
      setLastUpdatedTs(data.updated_ts || 0);
      toast.success(t("setting.memory-limit-section.saved"));
    } catch {
      toast.error(t("setting.memory-limit-section.save-failed"));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="w-full gap-2 pt-2 pb-4">
      <p className="font-medium text-gray-700 dark:text-gray-500">{t("setting.memory-limit-section.title")}</p>
      <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">{t("setting.memory-limit-section.description")}</p>

      <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
        <label className="col-span-1 flex items-center gap-2 text-sm text-gray-700 dark:text-gray-400">
          <input type="checkbox" checked={enabled} disabled={loading} onChange={(event) => setEnabled(event.target.checked)} />
          <span>{t("setting.memory-limit-section.enable")}</span>
        </label>

        <label className="col-span-1 text-sm text-gray-700 dark:text-gray-400">
          <span>{t("setting.memory-limit-section.limit-mb")}</span>
          <input
            type="number"
            min={128}
            max={65536}
            step={128}
            disabled={!enabled || loading}
            className="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-100 disabled:opacity-60 dark:border-zinc-700 dark:bg-zinc-900 dark:text-gray-200 dark:focus:border-blue-400 dark:focus:ring-zinc-700"
            value={memoryLimitMb}
            onChange={(event) => setMemoryLimitMb(Number(event.target.value))}
          />
        </label>
      </div>

      <p className="mt-2 text-xs text-gray-500 dark:text-gray-500">{t("setting.memory-limit-section.hint")}</p>
      <p className="mt-1 text-xs text-gray-500 dark:text-gray-500">
        {lastUpdatedTs > 0
          ? t("setting.memory-limit-section.last-updated", { time: new Date(lastUpdatedTs * 1000).toLocaleString() })
          : t("setting.memory-limit-section.not-set")}
      </p>

      <div className="mt-3">
        <Button color="primary" disabled={loading || saving || !hasValidInput} onClick={saveMemoryLimit}>
          {t("common.save")}
        </Button>
      </div>
    </div>
  );
};

export default MemoryLimitSection;
