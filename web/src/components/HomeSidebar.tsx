import { useEffect, useState } from "react";
import { LINK_REG } from "@/labs/marked/parser/Link";
import { PLAIN_LINK_REG } from "@/labs/marked/parser/PlainLink";
import { useFilterStore, useMemoStore, useTagStore, useUserStore } from "@/store/module";
import { useTranslate } from "@/utils/i18n";
import CalendarView from "./CalendarView";
import showCreateTagDialog from "./CreateTagDialog";
import Icon from "./Icon";
import SearchBar from "./SearchBar";

// Patterns for content-type counting
const TODO_REG = /- \[[ x]\] /g;
const TODO_DONE_REG = /- \[x\] /g;
const CODE_BLOCK_REG = /```[\s\S]*?```/g;
const INLINE_CODE_REG = /`[^`]+`/g;

const HomeSidebar = () => {
  const t = useTranslate();
  const filterStore = useFilterStore();
  const memoStore = useMemoStore();
  const tagStore = useTagStore();
  const userStore = useUserStore();

  const memos = memoStore.state.memos.filter((m) => m.creatorUsername === userStore.getCurrentUsername() && m.rowStatus === "NORMAL");
  const tagsText = tagStore.state.tags;
  const [tags, setTags] = useState<string[]>([]);

  useEffect(() => {
    tagStore.fetchTags();
  }, []);

  useEffect(() => {
    setTags(Array.from(tagsText).sort());
  }, [tagsText]);

  // Compute content-type counts
  const linkCount = memos.filter((m) => m.content.match(LINK_REG) || m.content.match(new RegExp(PLAIN_LINK_REG.source))).length;

  let todoTotal = 0;
  let todoDone = 0;
  for (const m of memos) {
    const allTodos = m.content.match(TODO_REG) ?? [];
    const doneTodos = m.content.match(TODO_DONE_REG) ?? [];
    todoTotal += allTodos.length;
    todoDone += doneTodos.length;
  }

  const codeCount = memos.filter((m) => m.content.match(CODE_BLOCK_REG) || m.content.match(INLINE_CODE_REG)).length;

  const activeType = filterStore.state.type;

  const handleLinkChip = () => {
    filterStore.setMemoTypeFilter(activeType === "LINKED" ? undefined : "LINKED");
  };

  const handleTagChipClick = (tag: string) => {
    const current = filterStore.state.tag;
    filterStore.setTagFilter(current === tag ? undefined : tag);
  };

  const activeTagFilter = filterStore.state.tag;

  return (
    <aside className="sticky top-0 w-72 shrink-0 h-screen overflow-y-auto hide-scrollbar flex flex-col justify-start items-start border-r border-gray-200 dark:border-zinc-700 bg-zinc-50 dark:bg-zinc-900 py-4 gap-1">
      {/* Search */}
      <div className="w-full px-4 mb-2">
        <SearchBar />
      </div>

      {/* Calendar */}
      <CalendarView />

      {/* Filter chips */}
      {!userStore.isVisitorMode() && (
        <div className="w-full px-3 flex flex-wrap gap-2 mb-1">
          <button
            onClick={handleLinkChip}
            className={`flex items-center gap-1 px-2.5 py-1 rounded-md border text-xs font-medium transition-colors ${
              activeType === "LINKED"
                ? "bg-blue-100 border-blue-300 text-blue-700 dark:bg-blue-900 dark:border-blue-600 dark:text-blue-200"
                : "bg-white border-gray-200 text-gray-600 dark:bg-zinc-800 dark:border-zinc-600 dark:text-gray-300 hover:border-gray-300"
            }`}
          >
            <Icon.Link2 className="w-3.5 h-auto" />
            Links {linkCount}
          </button>
          <span className="flex items-center gap-1 px-2.5 py-1 rounded-md border text-xs font-medium bg-white border-gray-200 text-gray-600 dark:bg-zinc-800 dark:border-zinc-600 dark:text-gray-300 select-none">
            <Icon.CheckSquare className="w-3.5 h-auto" />
            To-do {todoDone}/{todoTotal}
          </span>
          <span className="flex items-center gap-1 px-2.5 py-1 rounded-md border text-xs font-medium bg-white border-gray-200 text-gray-600 dark:bg-zinc-800 dark:border-zinc-600 dark:text-gray-300 select-none">
            <Icon.Code2 className="w-3.5 h-auto" />
            Code {codeCount}
          </span>
        </div>
      )}

      {/* Shortcuts */}
      {!userStore.isVisitorMode() && (
        <div className="w-full px-3 mb-1">
          <div className="flex items-center justify-between py-1">
            <span className="text-sm font-medium text-gray-600 dark:text-gray-300">Shortcuts</span>
            <span className="flex items-center justify-center w-6 h-6 rounded text-gray-300 dark:text-zinc-600 cursor-default">
              <Icon.Plus className="w-4 h-4" />
            </span>
          </div>
        </div>
      )}

      {/* Tags as chips */}
      {!userStore.isVisitorMode() && tags.length > 0 && (
        <div className="w-full px-3">
          <div className="flex items-center justify-between py-1 mb-1">
            <span className="text-sm font-medium text-gray-600 dark:text-gray-300">{t("common.tags")}</span>
            <button
              onClick={() => showCreateTagDialog()}
              className="flex items-center justify-center w-6 h-6 rounded hover:bg-gray-200 dark:hover:bg-zinc-700 text-gray-400"
            >
              <Icon.MoreVertical className="w-4 h-4" />
            </button>
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
  );
};

export default HomeSidebar;
