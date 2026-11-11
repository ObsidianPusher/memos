import { Tooltip } from "@mui/joy";
import { EarthIcon, LibraryIcon, PaperclipIcon, SmilePlusIcon, UserCircleIcon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { NavLink } from "react-router-dom";
import useCurrentUser from "@/hooks/useCurrentUser";
import { Routes } from "@/router/routes";
import { cn } from "@/utils";
import { useTranslate } from "@/utils/i18n";
import BrandBanner from "./BrandBanner";
import UserBanner from "./UserBanner";

interface NavLinkItem {
  id: string;
  path: string;
  title: string;
  icon: React.ReactNode;
}

interface Props {
  collapsed?: boolean;
  className?: string;
}

const Navigation = observer((props: Props) => {
  const { collapsed, className } = props;
  const t = useTranslate();
  const currentUser = useCurrentUser();
  const navLinks: NavLinkItem[] = currentUser
    ? [
        {
          id: "header-memos",
          path: Routes.ROOT,
          title: t("common.memos"),
          icon: <LibraryIcon className="w-6 h-auto opacity-70 shrink-0" />,
        },
        {
          id: "header-emotions",
          path: Routes.EMOTIONS,
          title: t("common.emotions"),
          icon: <SmilePlusIcon className="w-6 h-auto opacity-70 shrink-0" />,
        },
        {
          id: "header-explore",
          path: Routes.EXPLORE,
          title: t("common.explore"),
          icon: <EarthIcon className="w-6 h-auto opacity-70 shrink-0" />,
        },
        {
          id: "header-resources",
          path: Routes.RESOURCES,
          title: t("common.resources"),
          icon: <PaperclipIcon className="w-6 h-auto opacity-70 shrink-0" />,
        },
      ]
    : [
        {
          id: "header-explore",
          path: Routes.EXPLORE,
          title: t("common.explore"),
          icon: <EarthIcon className="w-6 h-auto opacity-70 shrink-0" />,
        },
        {
          id: "header-auth",
          path: Routes.AUTH,
          title: t("common.sign-in"),
          icon: <UserCircleIcon className="w-6 h-auto opacity-70 shrink-0" />,
        },
      ];

  return (
    <header
      className={cn(
        "w-full h-full overflow-auto flex flex-col justify-between items-start gap-4 py-4 md:pt-6 z-30 hide-scrollbar",
        className,
      )}
    >
      <div className="w-full px-1 py-1 flex flex-col justify-start items-start space-y-2 overflow-auto hide-scrollbar shrink">
        <NavLink className="mb-2 cursor-default" to={currentUser ? Routes.ROOT : Routes.EXPLORE}>
          <BrandBanner collapsed={collapsed} />
        </NavLink>
        {navLinks.map((navLink) => (
          <NavLink
            className={({ isActive }) =>
              cn(
                "px-2 py-2 rounded-2xl border flex flex-row items-center text-lg text-gray-800 dark:text-gray-400 hover:bg-white hover:border-gray-200 dark:hover:border-zinc-700 dark:hover:bg-zinc-800",
                collapsed ? "" : "w-full px-4",
                isActive ? "bg-white drop-shadow-sm dark:bg-zinc-800 border-gray-200 dark:border-zinc-700" : "border-transparent",
              )
            }
            key={navLink.id}
            to={navLink.path}
            id={navLink.id}
            viewTransition
          >
            {collapsed ? (
              <Tooltip title={navLink.title} placement="right" arrow>
                <div>{navLink.icon}</div>
              </Tooltip>
            ) : (
              <div className="flex items-center gap-3">
                <div>{navLink.icon}</div>
                <span className="truncate">{navLink.title}</span>
              </div>
            )}
          </NavLink>
        ))}
      </div>
      {currentUser && <UserBanner collapsed={collapsed} />}
    </header>
  );
});

export default Navigation;
