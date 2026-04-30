import { useState } from "react";
import { useLayoutStore } from "@/store/module";
import Icon from "./Icon";

interface Props {
  showSearch?: boolean;
  onMenuClick?: () => void;
  onSearchClick?: () => void;
}

const MobileHeader = (props: Props) => {
  const { showSearch = true, onMenuClick, onSearchClick } = props;
  const layoutStore = useLayoutStore();
  const [titleText] = useState("MEMOS");
  const handleNavigationClick =
    onMenuClick ??
    (() => {
      layoutStore.setHomeSidebarStatus(false);
      layoutStore.setHeaderStatus(true);
    });
  const handleSearchClick =
    onSearchClick ??
    (() => {
      layoutStore.setHeaderStatus(false);
      layoutStore.setHomeSidebarStatus(true);
    });

  return (
    <div className="sticky top-0 pt-4 sm:pt-1 pb-1 mb-1 backdrop-blur bg-zinc-100 dark:bg-zinc-800 bg-opacity-70 flex md:hidden flex-row justify-between items-center w-full h-auto flex-nowrap shrink-0 z-2">
      <div className="flex flex-row justify-start items-center mr-2 shrink-0 overflow-hidden">
        <button
          type="button"
          aria-label="Open navigation"
          aria-expanded={layoutStore.state.showHeader}
          className="flex sm:hidden flex-row justify-center items-center w-11 h-11 mr-1 shrink-0 bg-transparent"
          onClick={handleNavigationClick}
        >
          <Icon.Menu className="w-6 h-auto dark:text-gray-200" />
        </button>
        <span
          className="font-bold text-lg leading-10 mr-1 text-ellipsis shrink-0 cursor-pointer overflow-hidden text-gray-700 dark:text-gray-200"
          onClick={() => location.reload()}
        >
          {titleText}
        </span>
      </div>
      <button
        type="button"
        aria-label="Open search and calendar"
        aria-expanded={layoutStore.state.showHomeSidebar}
        className={`${showSearch ? "flex" : "hidden"} flex-row justify-center items-center w-11 h-11 shrink-0 bg-transparent`}
        onClick={handleSearchClick}
      >
        <Icon.Search className="w-6 h-auto dark:text-gray-200" />
      </button>
    </div>
  );
};

export default MobileHeader;
