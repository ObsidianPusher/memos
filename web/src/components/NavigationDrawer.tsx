import { Drawer } from "@mui/joy";
import { Button } from "@usememos/mui";
import { MenuIcon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useEffect, useState } from "react";
import { useLocation } from "react-router-dom";
import { workspaceStore } from "@/store/v2";
import Navigation from "./Navigation";
import UserAvatar from "./UserAvatar";

const NavigationDrawer = observer(() => {
  const location = useLocation();
  const [open, setOpen] = useState(false);
  const workspaceGeneralSetting = workspaceStore.state.generalSetting;
  const title = workspaceGeneralSetting.customProfile?.title || "Memos";
  const avatarUrl = workspaceGeneralSetting.customProfile?.logoUrl || "/full-logo.webp";

  useEffect(() => {
    setOpen(false);
  }, [location.pathname]);

  const toggleDrawer = (inOpen: boolean) => (event: React.KeyboardEvent | React.MouseEvent) => {
    if (event.type === "keydown" && ((event as React.KeyboardEvent).key === "Tab" || (event as React.KeyboardEvent).key === "Shift")) {
      return;
    }

    setOpen(inOpen);
  };

  return (
    <>
      <Button
        variant="plain"
        className="inline-flex items-center gap-2 rounded-xl border border-transparent bg-transparent px-2 py-2 text-gray-700 transition hover:bg-zinc-100 hover:text-gray-900 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 dark:text-gray-300 dark:hover:bg-zinc-800 dark:focus:ring-offset-zinc-900"
        onClick={toggleDrawer(true)}
        aria-label="打开导航菜单"
      >
        <MenuIcon className="w-6 h-6 shrink-0" />
        <UserAvatar className="hidden sm:inline-flex shrink-0 w-6 h-6 rounded-md" avatarUrl={avatarUrl} />
        <span className="hidden sm:inline-flex font-bold text-lg leading-10 ml-1 text-ellipsis shrink-0 cursor-pointer overflow-hidden text-gray-700 dark:text-gray-300">
          {title}
        </span>
      </Button>
      <Drawer anchor="left" size="sm" open={open} onClose={toggleDrawer(false)}>
        <div className="w-full h-full overflow-auto px-2 bg-zinc-100 dark:bg-zinc-900">
          <Navigation />
        </div>
      </Drawer>
    </>
  );
});

export default NavigationDrawer;
