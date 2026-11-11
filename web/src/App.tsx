import { useColorScheme } from "@mui/joy";
import { observer } from "mobx-react-lite";
import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Outlet } from "react-router-dom";
import { getSystemColorScheme } from "./helpers/utils";
import useNavigateTo from "./hooks/useNavigateTo";
import { Routes } from "./router/routes";
import { userStore, workspaceStore } from "./store/v2";

const App = observer(() => {
  const { i18n } = useTranslation();
  const navigateTo = useNavigateTo();
  const { mode, setMode } = useColorScheme();
  const workspaceProfile = workspaceStore.state.profile;
  const userSetting = userStore.state.userSetting;
  const workspaceGeneralSetting = workspaceStore.state.generalSetting;
  const isWorkspaceReady = workspaceStore.state.isInitialized;
  const isUserReady = userStore.state.isInitialized;

  // Redirect to sign up page if no instance owner.
  useEffect(() => {
    if (!isWorkspaceReady || !isUserReady) {
      return;
    }
    if (workspaceProfile.owner === "") {
      navigateTo(Routes.SIGN_UP, { replace: true });
    }
  }, [isUserReady, isWorkspaceReady, navigateTo, workspaceProfile.owner]);

  useEffect(() => {
    if (!workspaceGeneralSetting.additionalStyle) {
      return;
    }

    const styleEl = document.createElement("style");
    styleEl.textContent = workspaceGeneralSetting.additionalStyle;
    styleEl.setAttribute("type", "text/css");
    styleEl.setAttribute("data-memos-custom-style", "true");
    document.body.appendChild(styleEl);

    return () => {
      styleEl.remove();
    };
  }, [workspaceGeneralSetting.additionalStyle]);

  useEffect(() => {
    if (!workspaceGeneralSetting.additionalScript) {
      return;
    }

    const scriptEl = document.createElement("script");
    scriptEl.textContent = workspaceGeneralSetting.additionalScript;
    scriptEl.setAttribute("data-memos-custom-script", "true");
    document.head.appendChild(scriptEl);

    return () => {
      scriptEl.remove();
    };
  }, [workspaceGeneralSetting.additionalScript]);

  // Dynamic update metadata with customized profile.
  useEffect(() => {
    if (!workspaceGeneralSetting.customProfile) {
      return;
    }

    document.title = workspaceGeneralSetting.customProfile.title;
    const link = document.querySelector("link[rel~='icon']");
    if (link instanceof HTMLLinkElement) {
      link.href = workspaceGeneralSetting.customProfile.logoUrl || "/logo.webp";
    }
  }, [workspaceGeneralSetting.customProfile]);

  useEffect(() => {
    const currentLocale = workspaceStore.state.locale;
    // This will trigger re-rendering of the whole app.
    i18n.changeLanguage(currentLocale);
    document.documentElement.setAttribute("lang", currentLocale);
    if (["ar", "fa"].includes(currentLocale)) {
      document.documentElement.setAttribute("dir", "rtl");
    } else {
      document.documentElement.setAttribute("dir", "ltr");
    }
  }, [workspaceStore.state.locale]);

  useEffect(() => {
    const appearance = workspaceStore.state.appearance;
    const root = document.documentElement;
    root.dataset.theme = appearance;
    const applyMode = () =>
      setMode(appearance === "system" ? getSystemColorScheme() : appearance === "dracula" ? "dark" : (appearance as "light" | "dark"));
    applyMode();
    if (appearance !== "system") return;
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    media.addEventListener("change", applyMode);
    return () => media.removeEventListener("change", applyMode);
  }, [workspaceStore.state.appearance, setMode]);

  useEffect(() => {
    document.documentElement.classList.toggle("dark", mode === "dark");
  }, [mode]);

  useEffect(() => {
    document.documentElement.style.fontSize = `${workspaceStore.state.fontSize}px`;
  }, [workspaceStore.state.fontSize]);

  useEffect(() => {
    if (!userSetting) {
      return;
    }

    workspaceStore.state.setPartial({
      locale: userSetting.locale || workspaceStore.state.locale,
      appearance: userSetting.appearance || workspaceStore.state.appearance,
    });
  }, [userSetting?.locale, userSetting?.appearance]);

  return <Outlet />;
});

export default App;
