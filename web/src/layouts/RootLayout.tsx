import { observer } from "mobx-react-lite";
import { Suspense, useEffect, useMemo } from "react";
import { Navigate, Outlet, useLocation, useSearchParams } from "react-router-dom";
import Navigation from "@/components/Navigation";
import useCurrentUser from "@/hooks/useCurrentUser";
import useResponsiveWidth from "@/hooks/useResponsiveWidth";
import Loading from "@/pages/Loading";
import { ROOT_SECTION_ROUTES, Routes } from "@/router/routes";
import { workspaceStore } from "@/store/v2";
import memoFilterStore from "@/store/v2/memoFilter";
import { cn } from "@/utils";

const RootLayout = observer(() => {
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const { sm } = useResponsiveWidth();
  const currentUser = useCurrentUser();
  const pathname = useMemo(() => location.pathname, [location.pathname]);
  const isWorkspaceReady = workspaceStore.state.isInitialized;

  useEffect(() => {
    // When the route changes and there is no filter in the search params, remove all filters.
    if (location.pathname !== pathname && !searchParams.has("filter")) {
      memoFilterStore.removeFilter(() => true);
    }
  }, [location.pathname, pathname, searchParams]);

  if (!isWorkspaceReady) {
    return <Loading />;
  }

  if (!currentUser && workspaceStore.state.memoRelatedSetting.disallowPublicVisibility) {
    return <Navigate to={Routes.AUTH} replace />;
  }

  if (!currentUser && ROOT_SECTION_ROUTES.includes(location.pathname as (typeof ROOT_SECTION_ROUTES)[number])) {
    return <Navigate to={Routes.EXPLORE} replace />;
  }

  return (
    <div className="w-full min-h-full flex flex-col sm:flex-row justify-start items-stretch sm:pl-16">
      {sm && (
        <div
          className={cn(
            "group flex flex-col justify-start items-start fixed top-0 left-0 select-none border-r dark:border-zinc-800 h-full bg-zinc-100 dark:bg-zinc-800 dark:bg-opacity-40",
            "w-16 px-2",
          )}
        >
          <Navigation collapsed={true} />
        </div>
      )}
      <main className="min-w-0 w-full h-auto flex-grow shrink flex flex-col justify-start items-center">
        <Suspense fallback={<Loading />}>
          <Outlet />
        </Suspense>
      </main>
    </div>
  );
});

export default RootLayout;
