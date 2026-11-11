import { Suspense, lazy } from "react";
import { createBrowserRouter } from "react-router-dom";
import App from "@/App";
import RouterErrorBoundary from "@/components/RouterErrorBoundary";
import Loading from "@/pages/Loading";
import { lazyWithRetry } from "@/utils/lazyWithRetry";
import { Routes } from "./routes";

const AdminSignIn = lazy(lazyWithRetry(() => import("@/pages/AdminSignIn")));
const Archived = lazy(lazyWithRetry(() => import("@/pages/Archived")));
const AuthCallback = lazy(lazyWithRetry(() => import("@/pages/AuthCallback")));
const Explore = lazy(lazyWithRetry(() => import("@/pages/Explore")));
const Emotion = lazy(lazyWithRetry(() => import("@/pages/Emotion")));
const Inboxes = lazy(lazyWithRetry(() => import("@/pages/Inboxes")));
const Home = lazy(lazyWithRetry(() => import("@/pages/Home")));
const HomeLayout = lazy(lazyWithRetry(() => import("@/layouts/HomeLayout")));
const MemoDetail = lazy(lazyWithRetry(() => import("@/pages/MemoDetail")));
const NotFound = lazy(lazyWithRetry(() => import("@/pages/NotFound")));
const PermissionDenied = lazy(lazyWithRetry(() => import("@/pages/PermissionDenied")));
const Resources = lazy(lazyWithRetry(() => import("@/pages/Resources")));
const RootLayout = lazy(lazyWithRetry(() => import("@/layouts/RootLayout")));
const Setting = lazy(lazyWithRetry(() => import("@/pages/Setting")));
const SignIn = lazy(lazyWithRetry(() => import("@/pages/SignIn")));
const SignUp = lazy(lazyWithRetry(() => import("@/pages/SignUp")));
const UserProfile = lazy(lazyWithRetry(() => import("@/pages/UserProfile")));
const MemoDetailRedirect = lazy(lazyWithRetry(() => import("./MemoDetailRedirect")));

const router = createBrowserRouter([
  {
    path: "/",
    element: <App />,
    errorElement: <RouterErrorBoundary />,
    children: [
      {
        path: Routes.AUTH,
        children: [
          {
            index: true,
            element: (
              <Suspense fallback={<Loading />}>
                <SignIn />
              </Suspense>
            ),
          },
          {
            path: "admin",
            element: (
              <Suspense fallback={<Loading />}>
                <AdminSignIn />
              </Suspense>
            ),
          },
          {
            path: "signup",
            element: (
              <Suspense fallback={<Loading />}>
                <SignUp />
              </Suspense>
            ),
          },
          {
            path: "callback",
            element: (
              <Suspense fallback={<Loading />}>
                <AuthCallback />
              </Suspense>
            ),
          },
        ],
      },
      {
        path: Routes.ROOT,
        element: (
          <Suspense fallback={<Loading />}>
            <RootLayout />
          </Suspense>
        ),
        children: [
          {
            element: (
              <Suspense fallback={<Loading />}>
                <HomeLayout />
              </Suspense>
            ),
            children: [
              {
                index: true,
                element: (
                  <Suspense fallback={<Loading />}>
                    <Home />
                  </Suspense>
                ),
              },
              {
                path: Routes.ARCHIVED,
                element: (
                  <Suspense fallback={<Loading />}>
                    <Archived />
                  </Suspense>
                ),
              },
              {
                path: "u/:username",
                element: (
                  <Suspense fallback={<Loading />}>
                    <UserProfile />
                  </Suspense>
                ),
              },
            ],
          },
          {
            path: Routes.EMOTIONS,
            element: (
              <Suspense fallback={<Loading />}>
                <Emotion />
              </Suspense>
            ),
          },
          {
            path: Routes.EXPLORE,
            element: (
              <Suspense fallback={<Loading />}>
                <Explore />
              </Suspense>
            ),
          },
          {
            path: Routes.RESOURCES,
            element: (
              <Suspense fallback={<Loading />}>
                <Resources />
              </Suspense>
            ),
          },
          {
            path: Routes.INBOX,
            element: (
              <Suspense fallback={<Loading />}>
                <Inboxes />
              </Suspense>
            ),
          },
          {
            path: Routes.SETTING,
            element: (
              <Suspense fallback={<Loading />}>
                <Setting />
              </Suspense>
            ),
          },
          {
            path: Routes.MEMO_DETAIL,
            element: (
              <Suspense fallback={<Loading />}>
                <MemoDetail />
              </Suspense>
            ),
          },
          // Redirect old path to new path.
          {
            path: Routes.LEGACY_MEMO_DETAIL,
            element: (
              <Suspense fallback={<Loading />}>
                <MemoDetailRedirect />
              </Suspense>
            ),
          },
          {
            path: "403",
            element: (
              <Suspense fallback={<Loading />}>
                <PermissionDenied />
              </Suspense>
            ),
          },
          {
            path: "404",
            element: (
              <Suspense fallback={<Loading />}>
                <NotFound />
              </Suspense>
            ),
          },
          {
            path: "*",
            element: (
              <Suspense fallback={<Loading />}>
                <NotFound />
              </Suspense>
            ),
          },
        ],
      },
    ],
  },
]);

export default router;
export { Routes };
