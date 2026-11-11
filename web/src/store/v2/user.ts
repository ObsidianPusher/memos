import { uniqueId } from "lodash-es";
import { makeAutoObservable } from "mobx";
import { authServiceClient, inboxServiceClient, shortcutServiceClient, userServiceClient } from "@/grpcweb";
import { Inbox } from "@/types/proto/api/v1/inbox_service";
import { Shortcut } from "@/types/proto/api/v1/shortcut_service";
import { User, UserSetting, UserStats } from "@/types/proto/api/v1/user_service";
import { findNearestMatchedLanguage } from "@/utils/i18n";
import workspaceStore from "./workspace";

class LocalState {
  isInitialized = false;
  currentUser?: string;
  userSetting?: UserSetting;
  shortcuts: Shortcut[] = [];
  inboxes: Inbox[] = [];
  userMapByName: Record<string, User> = {};
  inflightUserByName: Record<string, Promise<User>> = {};
  inflightUserByUsername: Record<string, Promise<User>> = {};
  userStatsByName: Record<string, UserStats> = {};

  // The state id of user stats map.
  statsStateId = uniqueId();

  get tagCount() {
    const tagCount: Record<string, number> = {};
    for (const stats of Object.values(this.userStatsByName)) {
      for (const tag of Object.keys(stats.tagCount)) {
        tagCount[tag] = (tagCount[tag] || 0) + stats.tagCount[tag];
      }
    }
    return tagCount;
  }

  get currentUserStats() {
    if (!this.currentUser) {
      return undefined;
    }
    return this.userStatsByName[this.currentUser];
  }

  constructor() {
    makeAutoObservable(this);
  }

  setPartial(partial: Partial<LocalState>) {
    Object.assign(this, partial);
  }
}

const userStore = (() => {
  const state = new LocalState();

  const getOrFetchUserByName = async (name: string) => {
    if (state.userMapByName[name]) {
      return state.userMapByName[name] as User;
    }

    const inflight = state.inflightUserByName[name];
    if (inflight) {
      return inflight;
    }

    const request = userServiceClient
      .getUser({
        name,
      })
      .then((user) => {
        state.setPartial({
          userMapByName: {
            ...state.userMapByName,
            [name]: user,
          },
        });
        return user;
      })
      .finally(() => {
        const inflightMap = { ...state.inflightUserByName };
        delete inflightMap[name];
        state.setPartial({ inflightUserByName: inflightMap });
      });

    state.setPartial({
      inflightUserByName: {
        ...state.inflightUserByName,
        [name]: request,
      },
    });

    return request;
  };

  const getOrFetchUserByUsername = async (username: string) => {
    for (const name in state.userMapByName) {
      if (state.userMapByName[name].username === username) {
        return state.userMapByName[name];
      }
    }

    const inflight = state.inflightUserByUsername[username];
    if (inflight) {
      return inflight;
    }

    const request = userServiceClient
      .getUserByUsername({
        username,
      })
      .then((user) => {
        state.setPartial({
          userMapByName: {
            ...state.userMapByName,
            [user.name]: user,
          },
        });
        return user;
      })
      .finally(() => {
        const inflightMap = { ...state.inflightUserByUsername };
        delete inflightMap[username];
        state.setPartial({ inflightUserByUsername: inflightMap });
      });

    state.setPartial({
      inflightUserByUsername: {
        ...state.inflightUserByUsername,
        [username]: request,
      },
    });

    return request;
  };

  const getUserByName = (name: string) => {
    return state.userMapByName[name];
  };

  const fetchUsers = async () => {
    const { users } = await userServiceClient.listUsers({});
    const userMap = { ...state.userMapByName };
    for (const user of users) {
      userMap[user.name] = user;
    }
    state.setPartial({
      userMapByName: userMap,
    });
    return users;
  };

  const updateUser = async (user: Partial<User>, updateMask: string[]) => {
    const updatedUser = await userServiceClient.updateUser({
      user,
      updateMask,
    });
    state.setPartial({
      userMapByName: {
        ...state.userMapByName,
        [updatedUser.name]: updatedUser,
      },
    });
  };

  const deleteUser = async (name: string) => {
    await userServiceClient.deleteUser({ name });
    const userMap = state.userMapByName;
    delete userMap[name];
    state.setPartial({
      userMapByName: userMap,
    });
  };

  const updateUserSetting = async (userSetting: Partial<UserSetting>, updateMask: string[]) => {
    const updatedUserSetting = await userServiceClient.updateUserSetting({
      setting: userSetting,
      updateMask: updateMask,
    });
    state.setPartial({
      userSetting: UserSetting.fromPartial({
        ...state.userSetting,
        ...updatedUserSetting,
      }),
    });
  };

  const fetchShortcuts = async () => {
    if (!state.currentUser) {
      return;
    }

    const { shortcuts } = await shortcutServiceClient.listShortcuts({ parent: state.currentUser });
    state.setPartial({
      shortcuts,
    });
  };

  const fetchInboxes = async () => {
    const { inboxes } = await inboxServiceClient.listInboxes({});
    state.setPartial({
      inboxes,
    });
  };

  const updateInbox = async (inbox: Partial<Inbox>, updateMask: string[]) => {
    const updatedInbox = await inboxServiceClient.updateInbox({
      inbox,
      updateMask,
    });
    state.setPartial({
      inboxes: state.inboxes.map((i) => {
        if (i.name === updatedInbox.name) {
          return updatedInbox;
        }
        return i;
      }),
    });
    return updatedInbox;
  };

  const fetchUserStats = async (user?: string) => {
    const userStatsByName: Record<string, UserStats> = {};
    if (!user) {
      const { userStats } = await userServiceClient.listAllUserStats({});
      for (const stats of userStats) {
        userStatsByName[stats.name] = stats;
      }
    } else {
      const userStats = await userServiceClient.getUserStats({ name: user });
      userStatsByName[user] = userStats;
    }
    state.setPartial({
      userStatsByName: {
        ...userStatsByName,
      },
    });
  };

  const setStatsStateId = (id = uniqueId()) => {
    state.statsStateId = id;
  };

  const resetUserState = () => {
    state.setPartial({
      currentUser: undefined,
      userSetting: undefined,
      shortcuts: [],
      inboxes: [],
      userMapByName: {},
      userStatsByName: {},
      isInitialized: true,
    });
  };

  return {
    state,
    getOrFetchUserByName,
    getOrFetchUserByUsername,
    getUserByName,
    fetchUsers,
    updateUser,
    deleteUser,
    updateUserSetting,
    fetchShortcuts,
    fetchInboxes,
    updateInbox,
    fetchUserStats,
    setStatsStateId,
    resetUserState,
  };
})();

export const initialUserStore = async () => {
  try {
    const [currentUser, userSetting] = await Promise.all([authServiceClient.getAuthStatus({}), userServiceClient.getUserSetting({})]);
    userStore.state.setPartial({
      currentUser: currentUser.name,
      userSetting: UserSetting.fromPartial({
        ...userSetting,
      }),
      userMapByName: {
        [currentUser.name]: currentUser,
      },
      isInitialized: true,
    });
    workspaceStore.state.setPartial({
      locale: userSetting.locale,
      appearance: userSetting.appearance,
    });
  } catch {
    // find the nearest matched lang based on the `navigator.language` if the user is unauthenticated or settings retrieval fails.
    const locale = findNearestMatchedLanguage(navigator.language);
    workspaceStore.state.setPartial({
      locale: locale,
    });
    userStore.state.setPartial({
      isInitialized: true,
    });
  }
};

export default userStore;
