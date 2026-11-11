export const Routes = {
  ROOT: "/",
  RESOURCES: "/resources",
  INBOX: "/inbox",
  EMOTIONS: "/emotions",
  ARCHIVED: "/archived",
  SETTING: "/setting",
  EXPLORE: "/explore",
  AUTH: "/auth",
  SIGN_IN: "/auth",
  SIGN_UP: "/auth/signup",
  AUTH_CALLBACK: "/auth/callback",
  AUTH_ADMIN: "/auth/admin",
  MEMO_DETAIL: "/memos/:uid",
  LEGACY_MEMO_DETAIL: "/m/:uid",
  LEGACY_MEMO_DETAIL_PREFIX: "/m/",
  PERMISSION_DENIED: "/403",
  NOT_FOUND: "/404",
} as const;

export const PUBLIC_ROUTES = [Routes.EXPLORE, Routes.AUTH] as const;

export const ROOT_SECTION_ROUTES = [Routes.ROOT, Routes.RESOURCES, Routes.INBOX, Routes.EMOTIONS, Routes.ARCHIVED, Routes.SETTING] as const;
