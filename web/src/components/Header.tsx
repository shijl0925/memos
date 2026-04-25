import classNames from "classnames";
import { useEffect } from "react";
import { NavLink, useLocation } from "react-router-dom";
import { useGlobalStore, useLayoutStore, useUserStore } from "@/store/module";
import { useTranslate } from "@/utils/i18n";
import { resolution } from "@/utils/layout";
import Icon from "./Icon";
import UserAvatar from "./UserAvatar";

const Header = () => {
  const t = useTranslate();
  const location = useLocation();
  const globalStore = useGlobalStore();
  const userStore = useUserStore();
  const layoutStore = useLayoutStore();
  const showHeader = layoutStore.state.showHeader;
  const isVisitorMode = userStore.isVisitorMode() && !userStore.state.user;
  const { user } = userStore.state;

  useEffect(() => {
    const handleWindowResize = () => {
      if (window.innerWidth < resolution.sm) {
        layoutStore.setHeaderStatus(false);
      } else {
        layoutStore.setHeaderStatus(true);
      }
    };
    window.addEventListener("resize", handleWindowResize);
    handleWindowResize();
  }, [location]);

  const iconNavClass = ({ isActive }: { isActive: boolean }) =>
    classNames(
      "flex items-center justify-center w-10 h-10 rounded-xl text-gray-500 dark:text-gray-400 hover:bg-white dark:hover:bg-zinc-700 transition-colors",
      isActive && "bg-white dark:bg-zinc-700 shadow text-gray-800 dark:text-gray-100"
    );

  return (
    <div
      className={`fixed sm:sticky top-0 left-0 w-full sm:w-14 h-full shrink-0 pointer-events-none sm:pointer-events-auto z-10 ${
        showHeader && "pointer-events-auto"
      }`}
    >
      <div
        className={`fixed top-0 left-0 w-full h-full bg-black opacity-0 pointer-events-none transition-opacity duration-300 sm:!hidden ${
          showHeader && "opacity-60 pointer-events-auto"
        }`}
        onClick={() => layoutStore.setHeaderStatus(false)}
      />
      <header
        className={`relative w-14 sm:w-full h-full max-h-screen overflow-auto hide-scrollbar flex flex-col justify-between items-center py-4 z-30 bg-zinc-100 dark:bg-zinc-800 sm:bg-transparent sm:border-r sm:border-gray-200 sm:dark:border-zinc-700 sm:shadow-none transition-all duration-300 -translate-x-full sm:translate-x-0 ${
          showHeader && "translate-x-0 shadow-2xl"
        }`}
      >
        {/* Top: user avatar (display only) */}
        <div className="flex flex-col items-center">
          <div className="w-10 h-10 rounded-full overflow-clip" title={user?.nickname || user?.username || "Memos"}>
            <UserAvatar avatarUrl={user?.avatarUrl} className="w-10 h-10" />
          </div>
        </div>

        {/* Middle: navigation icons */}
        <nav className="flex flex-col items-center gap-1">
          {!isVisitorMode && (
            <>
              <NavLink to="/" id="header-home" title={t("common.home")} className={iconNavClass}>
                <Icon.Home className="w-5 h-auto" />
              </NavLink>
              <NavLink to="/resources" id="header-resources" title={t("common.resources")} className={iconNavClass}>
                <Icon.Paperclip className="w-5 h-auto" />
              </NavLink>
            </>
          )}
          {!globalStore.getDisablePublicMemos() && (
            <NavLink to="/explore" id="header-explore" title={t("common.explore")} className={iconNavClass}>
              <Icon.Hash className="w-5 h-auto" />
            </NavLink>
          )}
          {!isVisitorMode && (
            <NavLink to="/archived" id="header-archived" title={t("common.archived")} className={iconNavClass}>
              <Icon.Archive className="w-5 h-auto" />
            </NavLink>
          )}
        </nav>

        {/* Bottom: settings / sign-in */}
        <div className="flex flex-col items-center">
          {isVisitorMode ? (
            <NavLink to="/auth" id="header-auth" title={t("common.sign-in")} className={iconNavClass}>
              <Icon.LogIn className="w-5 h-auto" />
            </NavLink>
          ) : (
            <NavLink to="/setting" id="header-setting" title={t("common.settings")} className={iconNavClass}>
              <Icon.Settings className="w-5 h-auto" />
            </NavLink>
          )}
        </div>
      </header>
    </div>
  );
};

export default Header;
