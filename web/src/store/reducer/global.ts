import { createSlice, PayloadAction } from "@reduxjs/toolkit";
import { DEFAULT_MEMOS_LOGO_URL } from "@/helpers/consts";

interface State {
  locale: Locale;
  appearance: Appearance;
  systemStatus: SystemStatus;
}

const globalSlice = createSlice({
  name: "global",
  initialState: {
    locale: "en",
    appearance: "system",
    systemStatus: {
      host: undefined,
      profile: {
        mode: "dev",
        version: "",
      },
      dbSize: 0,
      allowSignUp: false,
      disablePasswordLogin: false,
      disablePublicMemos: false,
      additionalStyle: "",
      additionalScript: "",
      memoDisplayWithUpdatedTs: false,
      customizedProfile: {
        name: "memos",
        logoUrl: DEFAULT_MEMOS_LOGO_URL,
        description: "",
        locale: "en",
        appearance: "system",
        externalUrl: "",
      },
    },
  } as State,
  reducers: {
    setGlobalState: (state, action: PayloadAction<Partial<State>>) => {
      return {
        ...state,
        ...action.payload,
      };
    },
    setLocale: (state, action: PayloadAction<Locale>) => {
      return {
        ...state,
        locale: action.payload,
      };
    },
    setAppearance: (state, action: PayloadAction<Appearance>) => {
      return {
        ...state,
        appearance: action.payload,
      };
    },
  },
});

export const { setGlobalState, setLocale, setAppearance } = globalSlice.actions;

export default globalSlice.reducer;
