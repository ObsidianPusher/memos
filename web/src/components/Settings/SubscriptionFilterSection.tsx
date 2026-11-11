import { Button } from "@usememos/mui";
import copy from "copy-to-clipboard";
import { CopyIcon, ExternalLinkIcon, PencilIcon, PlusIcon, TrashIcon, XIcon } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { toast } from "react-hot-toast";
import { isValidUrl } from "@/helpers/utils";
import { useTranslate } from "@/utils/i18n";

const API_ENDPOINT = import.meta.env.VITE_API_ENDPOINT ?? window.location.origin;

interface ClashVergeSubscription {
  id: string;
  name: string;
  source_type: "url" | "content";
  source: string;
  content?: string;
  include: string;
  auto_update: boolean;
  update_interval_seconds: number;
  last_refresh_ts: number;
  last_refresh_error?: string;
  public_url: string;
  created_ts: number;
  updated_ts: number;
}

const SubscriptionFilterSection = () => {
  const t = useTranslate();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [editingId, setEditingId] = useState<string>("");
  const [subscriptionList, setSubscriptionList] = useState<ClashVergeSubscription[]>([]);
  const [name, setName] = useState("");
  const [sourceType, setSourceType] = useState<"url" | "content">("url");
  const [subscriptionURL, setSubscriptionURL] = useState("");
  const [subscriptionContent, setSubscriptionContent] = useState("");
  const [includeKeywords, setIncludeKeywords] = useState("");
  const [autoUpdate, setAutoUpdate] = useState(true);
  const [updateIntervalMinutes, setUpdateIntervalMinutes] = useState(60);

  const hasValidInput = useMemo(() => {
    const hasSource = sourceType === "url" ? isValidUrl(subscriptionURL.trim()) : subscriptionContent.trim() !== "";
    return hasSource && includeKeywords.trim() !== "";
  }, [sourceType, subscriptionURL, subscriptionContent, includeKeywords]);

  useEffect(() => {
    fetchSubscriptionList();
  }, []);

  const fetchSubscriptionList = async () => {
    setLoading(true);
    try {
      const response = await fetch(`${API_ENDPOINT}/api/v1/clash-verge/subscriptions`, {
        method: "GET",
        credentials: "include",
      });
      if (!response.ok) {
        throw new Error(await response.text());
      }
      const data = await response.json();
      setSubscriptionList((data.subscriptions || []) as ClashVergeSubscription[]);
    } catch {
      toast.error(t("setting.subscription-filter-section.load-failed"));
    } finally {
      setLoading(false);
    }
  };

  const resetForm = () => {
    setEditingId("");
    setName("");
    setSourceType("url");
    setSubscriptionURL("");
    setSubscriptionContent("");
    setIncludeKeywords("");
    setAutoUpdate(true);
    setUpdateIntervalMinutes(60);
  };

  const handleSubmit = async () => {
    if (!hasValidInput) {
      toast.error(t("setting.subscription-filter-section.invalid-input"));
      return;
    }
    setSaving(true);
    try {
      const response = await fetch(
        editingId
          ? `${API_ENDPOINT}/api/v1/clash-verge/subscriptions/${encodeURIComponent(editingId)}`
          : `${API_ENDPOINT}/api/v1/clash-verge/subscriptions`,
        {
          method: editingId ? "PATCH" : "POST",
          credentials: "include",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            name,
            source_type: sourceType,
            source: sourceType === "url" ? subscriptionURL.trim() : "",
            content: sourceType === "content" ? subscriptionContent.trim() : "",
            include: includeKeywords.trim(),
            auto_update: sourceType === "url" && autoUpdate,
            update_interval_seconds: sourceType === "url" && autoUpdate ? updateIntervalMinutes * 60 : 0,
          }),
        },
      );
      if (!response.ok) {
        throw new Error(await response.text());
      }
      await fetchSubscriptionList();
      toast.success(editingId ? t("setting.subscription-filter-section.updated") : t("setting.subscription-filter-section.created"));
      resetForm();
    } catch {
      toast.error(
        editingId ? t("setting.subscription-filter-section.update-failed") : t("setting.subscription-filter-section.create-failed"),
      );
    } finally {
      setSaving(false);
    }
  };

  const handleCopyRemoteConfigURL = (url: string) => {
    if (!url) {
      return;
    }
    copy(url);
    toast.success(t("setting.subscription-filter-section.link-copied"));
  };

  const handleDelete = async (subscription: ClashVergeSubscription) => {
    if (!window.confirm(t("setting.subscription-filter-section.delete-confirm", { name: subscription.name || subscription.id }))) {
      return;
    }
    try {
      const response = await fetch(`${API_ENDPOINT}/api/v1/clash-verge/subscriptions/${encodeURIComponent(subscription.id)}`, {
        method: "DELETE",
        credentials: "include",
      });
      if (!response.ok) {
        throw new Error(await response.text());
      }
      setSubscriptionList(subscriptionList.filter((item) => item.id !== subscription.id));
      if (editingId === subscription.id) {
        resetForm();
      }
      toast.success(t("setting.subscription-filter-section.deleted"));
    } catch {
      toast.error(t("setting.subscription-filter-section.delete-failed"));
    }
  };

  const handleEdit = (subscription: ClashVergeSubscription) => {
    setEditingId(subscription.id);
    setName(subscription.name);
    setSourceType(subscription.source_type || "url");
    setSubscriptionURL(subscription.source);
    setSubscriptionContent(subscription.content || "");
    setIncludeKeywords(subscription.include);
    setAutoUpdate(subscription.auto_update);
    setUpdateIntervalMinutes(Math.max(1, Math.floor((subscription.update_interval_seconds || 3600) / 60)));
  };

  return (
    <div className="w-full flex flex-col justify-start items-start gap-4 pt-2 pb-4">
      <div className="w-full">
        <div className="sm:flex sm:items-center sm:justify-between">
          <div className="sm:flex-auto space-y-1">
            <p className="font-medium text-gray-700 dark:text-gray-400">{t("setting.subscription-filter-section.title")}</p>
            <p className="text-sm text-gray-700 dark:text-gray-500">{t("setting.subscription-filter-section.description")}</p>
          </div>
        </div>

        <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
          <label className="col-span-1 text-sm text-gray-700 dark:text-gray-400">
            <span>{t("setting.subscription-filter-section.name")}</span>
            <input
              className="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-100 dark:border-zinc-700 dark:bg-zinc-900 dark:text-gray-200 dark:focus:border-blue-400 dark:focus:ring-zinc-700"
              placeholder={t("setting.subscription-filter-section.name-placeholder")}
              value={name}
              onChange={(event) => setName(event.target.value)}
            />
          </label>

          <label className="col-span-1 text-sm text-gray-700 dark:text-gray-400">
            <span>{t("setting.subscription-filter-section.import-mode")}</span>
            <select
              className="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-100 dark:border-zinc-700 dark:bg-zinc-900 dark:text-gray-200 dark:focus:border-blue-400 dark:focus:ring-zinc-700"
              value={sourceType}
              onChange={(event) => setSourceType(event.target.value as "url" | "content")}
            >
              <option value="url">{t("setting.subscription-filter-section.import-mode-url")}</option>
              <option value="content">{t("setting.subscription-filter-section.import-mode-content")}</option>
            </select>
          </label>

          {sourceType === "url" ? (
            <label className="col-span-1 sm:col-span-2 text-sm text-gray-700 dark:text-gray-400">
              <span>{t("setting.subscription-filter-section.subscription-url")}</span>
              <input
                className="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-100 dark:border-zinc-700 dark:bg-zinc-900 dark:text-gray-200 dark:focus:border-blue-400 dark:focus:ring-zinc-700"
                placeholder={t("setting.subscription-filter-section.subscription-url-placeholder")}
                value={subscriptionURL}
                onChange={(event) => setSubscriptionURL(event.target.value)}
              />
            </label>
          ) : (
            <label className="col-span-1 sm:col-span-2 text-sm text-gray-700 dark:text-gray-400">
              <span>{t("setting.subscription-filter-section.subscription-content")}</span>
              <textarea
                className="mt-1 min-h-40 w-full resize-y rounded-lg border border-gray-300 bg-white px-3 py-2 font-mono text-sm text-gray-900 shadow-sm outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-100 dark:border-zinc-700 dark:bg-zinc-900 dark:text-gray-200 dark:focus:border-blue-400 dark:focus:ring-zinc-700"
                placeholder={t("setting.subscription-filter-section.subscription-content-placeholder")}
                value={subscriptionContent}
                onChange={(event) => setSubscriptionContent(event.target.value)}
              />
              <span className="mt-1 block text-xs text-gray-500 dark:text-gray-500">
                {t("setting.subscription-filter-section.static-import-hint")}
              </span>
            </label>
          )}

          <label className="col-span-1 sm:col-span-2 text-sm text-gray-700 dark:text-gray-400">
            <span>{t("setting.subscription-filter-section.include-keywords")}</span>
            <input
              className="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-100 dark:border-zinc-700 dark:bg-zinc-900 dark:text-gray-200 dark:focus:border-blue-400 dark:focus:ring-zinc-700"
              placeholder={t("setting.subscription-filter-section.include-keywords-placeholder")}
              value={includeKeywords}
              onChange={(event) => setIncludeKeywords(event.target.value)}
            />
          </label>

          <label className="col-span-1 flex items-center gap-2 text-sm text-gray-700 dark:text-gray-400">
            <input
              type="checkbox"
              checked={sourceType === "url" && autoUpdate}
              disabled={sourceType === "content"}
              onChange={(event) => setAutoUpdate(event.target.checked)}
            />
            <span>{t("setting.subscription-filter-section.auto-update")}</span>
          </label>

          <label className="col-span-1 text-sm text-gray-700 dark:text-gray-400">
            <span>{t("setting.subscription-filter-section.update-interval")}</span>
            <select
              className="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-100 disabled:opacity-60 dark:border-zinc-700 dark:bg-zinc-900 dark:text-gray-200 dark:focus:border-blue-400 dark:focus:ring-zinc-700"
              value={updateIntervalMinutes}
              disabled={sourceType === "content" || !autoUpdate}
              onChange={(event) => setUpdateIntervalMinutes(Number(event.target.value))}
            >
              <option value={5}>5 {t("setting.subscription-filter-section.minutes")}</option>
              <option value={10}>10 {t("setting.subscription-filter-section.minutes")}</option>
              <option value={30}>30 {t("setting.subscription-filter-section.minutes")}</option>
              <option value={60}>60 {t("setting.subscription-filter-section.minutes")}</option>
              <option value={180}>180 {t("setting.subscription-filter-section.minutes")}</option>
              <option value={360}>360 {t("setting.subscription-filter-section.minutes")}</option>
              <option value={720}>720 {t("setting.subscription-filter-section.minutes")}</option>
              <option value={1440}>1440 {t("setting.subscription-filter-section.minutes")}</option>
            </select>
          </label>
        </div>

        <div className="mt-3 flex flex-wrap items-center gap-2">
          <Button color="primary" disabled={!hasValidInput || saving} onClick={handleSubmit}>
            <PlusIcon className="mr-1 h-4 w-4" />
            {editingId ? t("common.save") : t("common.create")}
          </Button>
          {editingId && (
            <Button variant="outlined" onClick={resetForm}>
              <XIcon className="mr-1 h-4 w-4" />
              {t("common.cancel")}
            </Button>
          )}
        </div>
        <p className="mt-2 text-xs text-gray-500 dark:text-gray-500">{t("setting.subscription-filter-section.security-hint")}</p>

        <div className="mt-4 overflow-x-auto rounded-lg border border-gray-200 dark:border-zinc-700">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-zinc-700">
            <thead className="bg-gray-50 dark:bg-zinc-900">
              <tr>
                <th className="px-3 py-2 text-left text-xs font-semibold text-gray-700 dark:text-gray-400">{t("common.title")}</th>
                <th className="px-3 py-2 text-left text-xs font-semibold text-gray-700 dark:text-gray-400">
                  {t("setting.subscription-filter-section.include-keywords")}
                </th>
                <th className="px-3 py-2 text-left text-xs font-semibold text-gray-700 dark:text-gray-400">
                  {t("setting.subscription-filter-section.generated-link")}
                </th>
                <th className="px-3 py-2 text-left text-xs font-semibold text-gray-700 dark:text-gray-400">
                  {t("setting.subscription-filter-section.update-policy")}
                </th>
                <th className="px-3 py-2 text-right text-xs font-semibold text-gray-700 dark:text-gray-400">
                  {t("setting.subscription-filter-section.actions")}
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 bg-white dark:divide-zinc-700 dark:bg-zinc-800">
              {!loading && subscriptionList.length === 0 && (
                <tr>
                  <td className="px-3 py-6 text-sm text-gray-500 dark:text-gray-400" colSpan={5}>
                    {t("setting.subscription-filter-section.empty")}
                  </td>
                </tr>
              )}
              {subscriptionList.map((subscription) => (
                <tr key={subscription.id}>
                  <td className="whitespace-nowrap px-3 py-2 text-sm text-gray-700 dark:text-gray-300">{subscription.name || "-"}</td>
                  <td className="max-w-60 truncate px-3 py-2 text-sm text-gray-600 dark:text-gray-400">{subscription.include}</td>
                  <td className="max-w-80 px-3 py-2 text-xs text-gray-600 dark:text-gray-400">
                    <div className="truncate font-mono">{subscription.public_url}</div>
                  </td>
                  <td className="px-3 py-2 text-xs text-gray-600 dark:text-gray-400">
                    {subscription.auto_update
                      ? t("setting.subscription-filter-section.interval-text", {
                          minutes: Math.max(1, Math.floor((subscription.update_interval_seconds || 3600) / 60)),
                        })
                      : t("setting.subscription-filter-section.no-update")}
                    <div>
                      {subscription.last_refresh_ts > 0
                        ? t("setting.subscription-filter-section.last-refresh", {
                            time: new Date(subscription.last_refresh_ts * 1000).toLocaleString(),
                          })
                        : t("setting.subscription-filter-section.never-refreshed")}
                    </div>
                    {subscription.last_refresh_error ? <div className="text-amber-600">{subscription.last_refresh_error}</div> : null}
                  </td>
                  <td className="whitespace-nowrap px-3 py-2 text-right">
                    <div className="inline-flex items-center gap-1">
                      <Button variant="plain" onClick={() => handleCopyRemoteConfigURL(subscription.public_url)}>
                        <CopyIcon className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="plain"
                        onClick={() => {
                          window.open(subscription.public_url, "_blank", "noopener,noreferrer");
                        }}
                      >
                        <ExternalLinkIcon className="h-4 w-4" />
                      </Button>
                      <Button variant="plain" onClick={() => handleEdit(subscription)}>
                        <PencilIcon className="h-4 w-4" />
                      </Button>
                      <Button variant="plain" onClick={() => handleDelete(subscription)}>
                        <TrashIcon className="h-4 w-4 text-red-600" />
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};

export default SubscriptionFilterSection;
