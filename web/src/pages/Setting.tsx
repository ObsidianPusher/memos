import { Option, Select } from "@mui/joy";
import {
  CogIcon,
  CpuIcon,
  DatabaseIcon,
  DownloadIcon,
  FilterIcon,
  KeyIcon,
  LibraryIcon,
  LucideIcon,
  Settings2Icon,
  UserIcon,
  UsersIcon,
} from "lucide-react";
import { observer } from "mobx-react-lite";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useLocation } from "react-router-dom";
import MobileHeader from "@/components/MobileHeader";
import ExportSection from "@/components/Settings/ExportSection";
import MemberSection from "@/components/Settings/MemberSection";
import MemoRelatedSettings from "@/components/Settings/MemoRelatedSettings";
import MemoryLimitSection from "@/components/Settings/MemoryLimitSection";
import MyAccountSection from "@/components/Settings/MyAccountSection";
import PreferencesSection from "@/components/Settings/PreferencesSection";
import SSOSection from "@/components/Settings/SSOSection";
import SectionMenuItem from "@/components/Settings/SectionMenuItem";
import StorageSection from "@/components/Settings/StorageSection";
import SubscriptionFilterSection from "@/components/Settings/SubscriptionFilterSection";
import WorkspaceSection from "@/components/Settings/WorkspaceSection";
import useCurrentUser from "@/hooks/useCurrentUser";
import useResponsiveWidth from "@/hooks/useResponsiveWidth";
import { workspaceStore } from "@/store/v2";
import { WorkspaceSettingKey } from "@/store/v2/workspace";
import { User_Role } from "@/types/proto/api/v1/user_service";
import { useTranslate } from "@/utils/i18n";

type SettingSection =
  | "my-account"
  | "preference"
  | "subscription-filter"
  | "export"
  | "member"
  | "system"
  | "memory-limit"
  | "memo-related"
  | "storage"
  | "sso";

interface State {
  selectedSection: SettingSection;
}

const BASIC_SECTIONS: SettingSection[] = ["my-account", "preference", "subscription-filter", "export"];
const ADMIN_SECTIONS: SettingSection[] = ["member", "system", "memory-limit", "memo-related", "storage", "sso"];
const SECTION_ICON_MAP: Record<SettingSection, LucideIcon> = {
  "my-account": UserIcon,
  preference: CogIcon,
  "subscription-filter": FilterIcon,
  export: DownloadIcon,
  member: UsersIcon,
  system: Settings2Icon,
  "memory-limit": CpuIcon,
  "memo-related": LibraryIcon,
  storage: DatabaseIcon,
  sso: KeyIcon,
};

const Setting = observer(() => {
  const t = useTranslate();
  const { md } = useResponsiveWidth();
  const location = useLocation();
  const user = useCurrentUser();
  const [state, setState] = useState<State>({
    selectedSection: "my-account",
  });
  const isHost = user.role === User_Role.HOST;

  const settingsSectionList = useMemo(() => {
    let settingList = [...BASIC_SECTIONS];
    if (isHost) {
      settingList = settingList.concat(ADMIN_SECTIONS);
    }
    return settingList;
  }, [isHost]);

  useEffect(() => {
    let hash = location.hash.slice(1) as SettingSection;
    // If the hash is not a valid section, redirect to the default section.
    if (![...BASIC_SECTIONS, ...ADMIN_SECTIONS].includes(hash)) {
      hash = "my-account";
    }
    setState({
      selectedSection: hash,
    });
  }, [location.hash]);

  useEffect(() => {
    if (!isHost) {
      return;
    }

    // Initial fetch for workspace settings.
    void Promise.all(
      [WorkspaceSettingKey.MEMO_RELATED, WorkspaceSettingKey.STORAGE].map(async (key) => {
        try {
          await workspaceStore.fetchWorkspaceSetting(key);
        } catch {
          // Settings are lazily created, so a missing record is equivalent to its default value.
        }
      }),
    );
  }, [isHost]);

  const handleSectionSelectorItemClick = useCallback((settingSection: SettingSection) => {
    window.location.hash = settingSection;
  }, []);

  return (
    <section className="@container w-full max-w-5xl min-h-full flex flex-col justify-start items-start sm:pt-3 md:pt-6 pb-8">
      {!md && <MobileHeader />}
      <div className="w-full px-4 sm:px-6">
        <div className="w-full shadow flex flex-row justify-start items-start px-4 py-3 rounded-xl bg-white dark:bg-zinc-800 text-gray-600 dark:text-gray-400">
          <div className="hidden sm:flex flex-col justify-start items-start w-40 h-auto shrink-0 py-2">
            <span className="text-sm mt-0.5 pl-3 font-mono select-none text-gray-400 dark:text-gray-500">{t("common.basic")}</span>
            <div className="w-full flex flex-col justify-start items-start mt-1">
              {BASIC_SECTIONS.map((item) => (
                <SectionMenuItem
                  key={item}
                  text={t(`setting.${item}`)}
                  icon={SECTION_ICON_MAP[item]}
                  isSelected={state.selectedSection === item}
                  onClick={() => handleSectionSelectorItemClick(item)}
                />
              ))}
            </div>
            {isHost ? (
              <>
                <span className="text-sm mt-4 pl-3 font-mono select-none text-gray-400 dark:text-gray-500">{t("common.admin")}</span>
                <div className="w-full flex flex-col justify-start items-start mt-1">
                  {ADMIN_SECTIONS.map((item) => (
                    <SectionMenuItem
                      key={item}
                      text={t(`setting.${item}`)}
                      icon={SECTION_ICON_MAP[item]}
                      isSelected={state.selectedSection === item}
                      onClick={() => handleSectionSelectorItemClick(item)}
                    />
                  ))}
                  <span className="px-3 mt-2 opacity-70 text-sm">
                    {t("setting.version")}: v{workspaceStore.state.profile.version}
                  </span>
                </div>
              </>
            ) : null}
          </div>
          <div className="w-full grow sm:pl-4 overflow-x-auto">
            <div className="w-auto inline-block my-2 sm:hidden">
              <Select value={state.selectedSection} onChange={(_, value) => handleSectionSelectorItemClick(value as SettingSection)}>
                {settingsSectionList.map((settingSection) => (
                  <Option key={settingSection} value={settingSection}>
                    {t(`setting.${settingSection}`)}
                  </Option>
                ))}
              </Select>
            </div>
            {state.selectedSection === "my-account" ? (
              <MyAccountSection />
            ) : state.selectedSection === "preference" ? (
              <PreferencesSection />
            ) : state.selectedSection === "subscription-filter" ? (
              <SubscriptionFilterSection />
            ) : state.selectedSection === "export" ? (
              <ExportSection />
            ) : state.selectedSection === "member" ? (
              <MemberSection />
            ) : state.selectedSection === "system" ? (
              <WorkspaceSection />
            ) : state.selectedSection === "memory-limit" ? (
              <MemoryLimitSection />
            ) : state.selectedSection === "memo-related" ? (
              <MemoRelatedSettings />
            ) : state.selectedSection === "storage" ? (
              <StorageSection />
            ) : state.selectedSection === "sso" ? (
              <SSOSection />
            ) : null}
          </div>
        </div>
      </div>
    </section>
  );
});

export default Setting;
