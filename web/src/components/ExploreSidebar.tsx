import { useMemo } from "react";
import { TAG_REG } from "@/labs/marked/parser";
import { useFilterStore, useLayoutStore, useMemoStore } from "@/store/module";
import { useTranslate } from "@/utils/i18n";
import CalendarView from "./CalendarView";
import SearchBar from "./SearchBar";

const ExploreSidebar = () => {
  const t = useTranslate();
  const filterStore = useFilterStore();
  const layoutStore = useLayoutStore();
  const memoStore = useMemoStore();
  const showHomeSidebar = layoutStore.state.showHomeSidebar;

  // Derive visible public/protected memos to build the tag list
  const publicMemos = memoStore.state.memos.filter((m) => m.rowStatus === "NORMAL" && m.visibility !== "PRIVATE");

  const tags = useMemo(() => {
    const tagSet = new Set<string>();
    for (const memo of publicMemos) {
      for (const match of Array.from(memo.content.matchAll(new RegExp(TAG_REG.source, "g")))) {
        const tag = match[1].trim();
        const parts = tag.split("/");
        let prefix = "";
        for (const part of parts) {
          prefix += part;
          tagSet.add(prefix);
          prefix += "/";
        }
      }
    }
    return Array.from(tagSet).sort();
  }, [publicMemos]);

  const activeTagFilter = filterStore.state.tag;

  const handleTagChipClick = (tag: string) => {
    filterStore.setTagFilter(activeTagFilter === tag ? undefined : tag);
    layoutStore.setHomeSidebarStatus(false);
  };

  return (
    <>
      <div
        className={`fixed inset-0 z-30 bg-black opacity-60 md:hidden ${showHomeSidebar ? "block" : "hidden"}`}
        onClick={() => layoutStore.setHomeSidebarStatus(false)}
      />
      <aside
        className={`fixed md:sticky top-0 left-0 z-40 md:z-auto w-72 max-w-[85vw] md:max-w-none shrink-0 h-full md:h-screen overflow-y-auto hide-scrollbar flex flex-col justify-start items-start border-r border-gray-200 dark:border-zinc-700 bg-zinc-50 dark:bg-zinc-900 py-4 gap-1 transition-transform duration-300 md:translate-x-0 ${
          showHomeSidebar ? "translate-x-0 shadow-2xl md:shadow-none" : "-translate-x-full"
        }`}
      >
        {/* Search */}
        <div className="w-full px-4 mb-2">
          <SearchBar />
        </div>

        {/* Calendar */}
        <CalendarView />

        {/* Tags as chips */}
        {tags.length > 0 && (
          <div className="w-full px-3 mt-1">
            <div className="flex items-center py-1 mb-1">
              <span className="text-sm font-medium text-gray-600 dark:text-gray-300">{t("common.tags")}</span>
            </div>
            <div className="flex flex-wrap gap-1.5">
              {tags.map((tag) => (
                <button
                  key={tag}
                  onClick={() => handleTagChipClick(tag)}
                  className={`flex items-center gap-0.5 px-2 py-0.5 rounded-full text-xs transition-colors ${
                    activeTagFilter === tag
                      ? "bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-200"
                      : "bg-gray-100 text-gray-600 dark:bg-zinc-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-zinc-600"
                  }`}
                >
                  <span className="opacity-60">#</span>
                  {tag}
                </button>
              ))}
            </div>
          </div>
        )}
      </aside>
    </>
  );
};

export default ExploreSidebar;
