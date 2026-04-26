import axios from "axios";

const csrfCookieName = "_csrf";
const csrfHeaderName = "X-CSRF-Token";

const getCookieValue = (name: string) => {
  const cookie = document.cookie.split("; ").find((row) => row.startsWith(`${encodeURIComponent(name)}=`));
  return cookie ? decodeURIComponent(cookie.split("=").slice(1).join("=")) : "";
};

axios.interceptors.request.use((config) => {
  const method = config.method?.toUpperCase() ?? "GET";
  const isSafeMethod = ["GET", "HEAD", "OPTIONS", "TRACE"].includes(method);
  const url = config.url ?? "";
  const isOwnAPI = url.startsWith("/api");
  if (!isSafeMethod && isOwnAPI) {
    const csrfToken = getCookieValue(csrfCookieName);
    if (csrfToken) {
      config.headers = config.headers ?? {};
      config.headers[csrfHeaderName] = csrfToken;
    }
  }
  return config;
});

// ---------------------------------------------------------------------------
// Axios response interceptor: unwrap the { data: T } envelope that the v0.12.2
// backend wraps every API response in.  External calls (e.g. GitHub API) are
// NOT affected because they use absolute HTTPS URLs.
// ---------------------------------------------------------------------------
axios.interceptors.response.use(
  (response) => {
    const url = response.config.url ?? "";
    // Only process relative URLs that target our own backend
    if (url.startsWith("/api") && response.data !== null && typeof response.data === "object" && "data" in response.data) {
      response.data = (response.data as { data: unknown }).data;
    }
    return response;
  },
  (error) => Promise.reject(error)
);

// ---------------------------------------------------------------------------
// Transform a raw memo object coming from the backend into the shape expected
// by the v0.14.4 frontend.
//   - creatorUsername : not in v0.12.2; fall back to creatorName
//   - displayTs       : not in v0.12.2; fall back to createdTs
//   - relationList    : not in v0.12.2; default to []
// ---------------------------------------------------------------------------
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const normalizeMemo = (m: any): any => ({
  ...m,
  creatorUsername: m.creatorUsername ?? m.creatorName ?? "",
  displayTs: m.displayTs ?? m.createdTs ?? 0,
  relationList: m.relationList ?? [],
});

// ---------------------------------------------------------------------------
// Lightweight in-process user cache: id → User.
// Populated lazily on first call to resolveCreatorId / getUserByUsername.
// ---------------------------------------------------------------------------
let userListCache: User[] | null = null;

const fetchUserList = async (): Promise<User[]> => {
  if (userListCache !== null) return userListCache;
  const { data } = await axios.get<User[]>("/api/user");
  // data is already unwrapped to User[] by the interceptor
  userListCache = Array.isArray(data) ? (data as User[]) : [];
  return userListCache;
};

/**
 * Resolve a username to a numeric creator ID by fetching the user list.
 * Returns undefined when the username is not found.
 */
const resolveCreatorId = async (username: string): Promise<number | undefined> => {
  const users = await fetchUserList();
  return users.find((u) => u.username === username)?.id;
};

export function getSystemStatus() {
  return axios.get<SystemStatus>("/api/status");
}

export function getSystemSetting() {
  return axios.get<SystemSetting[]>("/api/system/setting");
}

export function upsertSystemSetting(systemSetting: SystemSetting) {
  return axios.post<SystemSetting>("/api/system/setting", systemSetting);
}

export function vacuumDatabase() {
  return axios.post("/api/system/vacuum");
}

export function signin(username: string, password: string) {
  return axios.post("/api/auth/signin", {
    username,
    password,
  });
}

export function signinWithSSO(identityProviderId: IdentityProviderId, code: string, redirectUri: string) {
  return axios.post("/api/auth/signin/sso", {
    identityProviderId,
    code,
    redirectUri,
  });
}

export function signup(username: string, password: string) {
  return axios.post("/api/auth/signup", {
    username,
    password,
  });
}

export function signout() {
  return axios.post("/api/auth/signout");
}

export function createUser(userCreate: UserCreate) {
  return axios.post<User>("/api/user", userCreate);
}

export function getMyselfUser() {
  return axios.get<User>("/api/user/me");
}

export function getUserList() {
  // Invalidate cache on explicit list fetch
  userListCache = null;
  return axios.get<User[]>("/api/user");
}

/**
 * Simulates the v0.14.4 `/api/v1/user/name/:username` endpoint.
 * Fetches the full user list from the backend and returns the matching user.
 * Returns a promise that resolves to an axios-like response object so call
 * sites can use the same `const { data } = await getUserByUsername(...)` pattern.
 */
export async function getUserByUsername(username: string): Promise<{ data: User }> {
  const users = await fetchUserList();
  const user = users.find((u) => u.username === username);
  if (!user) {
    return Promise.reject(new Error(`User not found: ${username}`));
  }
  return { data: user };
}

export function upsertUserSetting(upsert: UserSettingUpsert) {
  return axios.post<UserSetting>(`/api/user/setting`, upsert);
}

export function patchUser(userPatch: UserPatch) {
  return axios.patch<User>(`/api/user/${userPatch.id}`, userPatch);
}

export function deleteUser(userDelete: UserDelete) {
  return axios.delete(`/api/user/${userDelete.id}`);
}

export async function getAllMemos(memoFind?: MemoFind) {
  const queryList = [];
  if (memoFind?.offset) {
    queryList.push(`offset=${memoFind.offset}`);
  }
  if (memoFind?.limit) {
    queryList.push(`limit=${memoFind.limit}`);
  }

  if (memoFind?.creatorUsername) {
    const creatorId = await resolveCreatorId(memoFind.creatorUsername);
    if (creatorId !== undefined) {
      queryList.push(`creatorId=${creatorId}`);
    }
  }

  const res = await axios.get<Memo[]>(`/api/memo/all?${queryList.join("&")}`);
  res.data = (Array.isArray(res.data) ? res.data : []).map(normalizeMemo) as Memo[];
  return res;
}

export async function getMemoList(memoFind?: MemoFind) {
  const queryList = [];
  if (memoFind?.creatorUsername) {
    const creatorId = await resolveCreatorId(memoFind.creatorUsername);
    if (creatorId !== undefined) {
      queryList.push(`creatorId=${creatorId}`);
    }
  }
  if (memoFind?.rowStatus) {
    queryList.push(`rowStatus=${memoFind.rowStatus}`);
  }
  if (memoFind?.pinned) {
    queryList.push(`pinned=${memoFind.pinned}`);
  }
  if (memoFind?.offset) {
    queryList.push(`offset=${memoFind.offset}`);
  }
  if (memoFind?.limit) {
    queryList.push(`limit=${memoFind.limit}`);
  }
  const res = await axios.get<Memo[]>(`/api/memo?${queryList.join("&")}`);
  res.data = (Array.isArray(res.data) ? res.data : []).map(normalizeMemo) as Memo[];
  return res;
}

export async function getMemoStats(username: string) {
  const creatorId = await resolveCreatorId(username);
  if (creatorId === undefined) {
    return { data: [] as number[] };
  }
  return axios.get<number[]>(`/api/memo/stats?creatorId=${creatorId}`);
}

export async function getMemoById(id: MemoId) {
  const res = await axios.get<Memo>(`/api/memo/${id}`);
  res.data = normalizeMemo(res.data) as Memo;
  return res;
}

export function createMemo(memoCreate: MemoCreate) {
  return axios.post<Memo>("/api/memo", memoCreate);
}

export function patchMemo(memoPatch: MemoPatch) {
  return axios.patch<Memo>(`/api/memo/${memoPatch.id}`, memoPatch);
}

export function pinMemo(memoId: MemoId) {
  return axios.post(`/api/memo/${memoId}/organizer`, {
    pinned: true,
  });
}

export function unpinMemo(memoId: MemoId) {
  return axios.post(`/api/memo/${memoId}/organizer`, {
    pinned: false,
  });
}

export function deleteMemo(memoId: MemoId) {
  return axios.delete(`/api/memo/${memoId}`);
}

export function getResourceList() {
  return axios.get<Resource[]>("/api/resource");
}

export function getResourceListWithLimit(resourceFind?: ResourceFind) {
  const queryList = [];
  if (resourceFind?.offset) {
    queryList.push(`offset=${resourceFind.offset}`);
  }
  if (resourceFind?.limit) {
    queryList.push(`limit=${resourceFind.limit}`);
  }
  return axios.get<Resource[]>(`/api/resource?${queryList.join("&")}`);
}

export function createResource(resourceCreate: ResourceCreate) {
  return axios.post<Resource>("/api/resource", resourceCreate);
}

export function createResourceWithBlob(formData: FormData) {
  return axios.post<Resource>("/api/resource/blob", formData);
}

export function patchResource(resourcePatch: ResourcePatch) {
  return axios.patch<Resource>(`/api/resource/${resourcePatch.id}`, resourcePatch);
}

export function deleteResourceById(id: ResourceId) {
  return axios.delete(`/api/resource/${id}`);
}

export function getMemoResourceList(memoId: MemoId) {
  return axios.get<Resource[]>(`/api/memo/${memoId}/resource`);
}

export function upsertMemoResource(memoId: MemoId, resourceId: ResourceId) {
  return axios.post(`/api/memo/${memoId}/resource`, {
    resourceId,
  });
}

export function deleteMemoResource(memoId: MemoId, resourceId: ResourceId) {
  return axios.delete(`/api/memo/${memoId}/resource/${resourceId}`);
}

export function getTagList(tagFind?: TagFind) {
  void tagFind;
  // The v0.12.2 backend tag endpoint uses the authenticated user from the JWT;
  // it does not accept a creatorId/creatorUsername query parameter.  In visitor
  // mode this means we will show the logged-in user's tags instead of the
  // visitor's tags, which is an acceptable limitation without a backend change.
  return axios.get<string[]>(`/api/tag`);
}

export function getTagSuggestionList() {
  return axios.get<string[]>(`/api/tag/suggestion`);
}

export function upsertTag(tagName: string) {
  return axios.post<string>(`/api/tag`, {
    name: tagName,
  });
}

export function deleteTag(tagName: string) {
  return axios.post(`/api/tag/delete`, {
    name: tagName,
  });
}

export function getStorageList() {
  return axios.get<ObjectStorage[]>(`/api/storage`);
}

export function createStorage(storageCreate: StorageCreate) {
  return axios.post<ObjectStorage>(`/api/storage`, storageCreate);
}

export function patchStorage(storagePatch: StoragePatch) {
  return axios.patch<ObjectStorage>(`/api/storage/${storagePatch.id}`, storagePatch);
}

export function deleteStorage(storageId: StorageId) {
  return axios.delete(`/api/storage/${storageId}`);
}

export function getIdentityProviderList() {
  return axios.get<IdentityProvider[]>(`/api/idp`);
}

export function createIdentityProvider(identityProviderCreate: IdentityProviderCreate) {
  return axios.post<IdentityProvider>(`/api/idp`, identityProviderCreate);
}

export function patchIdentityProvider(identityProviderPatch: IdentityProviderPatch) {
  return axios.patch<IdentityProvider>(`/api/idp/${identityProviderPatch.id}`, identityProviderPatch);
}

export function deleteIdentityProvider(id: IdentityProviderId) {
  return axios.delete(`/api/idp/${id}`);
}

export async function getRepoStarCount() {
  const { data } = await axios.get(`https://api.github.com/repos/usememos/memos`, {
    headers: {
      Accept: "application/vnd.github.v3.star+json",
      Authorization: "",
    },
  });
  return data.stargazers_count as number;
}

export async function getRepoLatestTag() {
  const { data } = await axios.get(`https://api.github.com/repos/usememos/memos/tags`, {
    headers: {
      Accept: "application/vnd.github.v3.star+json",
      Authorization: "",
    },
  });
  return data[0].name as string;
}
