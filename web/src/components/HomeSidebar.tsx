import { useEffect, useState } from "react";
import { toast } from "react-hot-toast";
import * as api from "@/helpers/api";
import { DAILY_TIMESTAMP } from "@/helpers/consts";
import { getDateStampByDate } from "@/helpers/datetime";
import { LINK_REG } from "@/labs/marked/parser/Link";
import { PLAIN_LINK_REG } from "@/labs/marked/parser/PlainLink";
import { useFilterStore, useMemoStore, useTagStore, useUserStore } from "@/store/module";
import { useTranslate } from "@/utils/i18n";
import CalendarView from "./CalendarView";
import showCreateShortcutDialog from "./CreateShortcutDialog";
import showCreateTagDialog from "./CreateTagDialog";
import { showCommonDialog } from "./Dialog/CommonDialog";
import Icon from "./Icon";
import SearchBar from "./SearchBar";

// Patterns for content-type counting
const TODO_REG = /- \[[ x]\] /g;
const TODO_DONE_REG = /- \[x\] /g;
const CODE_BLOCK_REG = /```[\s\S]*?```/g;
const INLINE_CODE_REG = /`[^`]+`/g;
const VALID_MEMO_TYPES = new Set<MemoSpecType>(["NOT_TAGGED", "LINKED", "TODO", "CODE"]);
const VALID_VISIBILITIES = new Set<Visibility>(["PUBLIC", "PROTECTED", "PRIVATE"]);

const normalizeMemoType = (value: string): MemoSpecType | undefined => {
  const normalized = value.trim().toUpperCase().replaceAll("-", "_");
  if (normalized === "HASLINK" || normalized === "HAS_LINK" || normalized === "LINK") {
    return "LINKED";
  }
  if (normalized === "HASTASKLIST" || normalized === "HAS_TASK_LIST" || normalized === "TASK" || normalized === "TASK_LIST") {
    return "TODO";
  }
  if (normalized === "HASCODE" || normalized === "HAS_CODE") {
    return "CODE";
  }
  return VALID_MEMO_TYPES.has(normalized as MemoSpecType) ? (normalized as MemoSpecType) : undefined;
};

const parseLegacyShortcutFilters = (payload: string): Partial<ReturnType<typeof useFilterStore>["state"]> | undefined => {
  try {
    const filters = JSON.parse(payload) as Filter[];
    if (!Array.isArray(filters)) {
      return undefined;
    }

    const parsed: Partial<ReturnType<typeof useFilterStore>["state"]> = {};
    let from: number | undefined;
    let to: number | undefined;
    for (const filter of filters) {
      if (filter.type === "TAG" && filter.value.operator === "CONTAIN") {
        parsed.tag = filter.value.value;
      } else if (filter.type === "TEXT" && filter.value.operator === "CONTAIN") {
        parsed.text = filter.value.value;
      } else if (filter.type === "TYPE" && filter.value.operator === "IS") {
        parsed.type = normalizeMemoType(filter.value.value);
      } else if (filter.type === "VISIBILITY" && filter.value.operator === "IS") {
        const visibility = filter.value.value.toUpperCase();
        if (VALID_VISIBILITIES.has(visibility as Visibility)) {
          parsed.visibility = visibility as Visibility;
        }
      } else if (filter.type === "DISPLAY_TIME") {
        const timestamp = getDateStampByDate(filter.value.value);
        if (filter.value.operator === "AFTER") {
          from = timestamp;
        } else if (filter.value.operator === "BEFORE") {
          to = timestamp;
        }
      }
    }
    if (from !== undefined && to !== undefined && from < to) {
      parsed.duration = { from, to };
    }
    return parsed;
  } catch {
    return undefined;
  }
};

const parseShortcutPayload = (payload: string): Partial<ReturnType<typeof useFilterStore>["state"]> => {
  const legacyFilters = parseLegacyShortcutFilters(payload);
  if (legacyFilters) {
    return legacyFilters;
  }

  const parsed: Partial<ReturnType<typeof useFilterStore>["state"]> = {};
  const filters = payload
    .split(/[,\n]/)
    .map((item) => item.trim())
    .filter(Boolean);

  for (const filter of filters) {
    const separatorIndex = filter.indexOf(":");
    if (separatorIndex < 0) {
      continue;
    }
    const factor = filter.slice(0, separatorIndex).trim().toLowerCase();
    const value = decodeURIComponent(filter.slice(separatorIndex + 1).trim());
    if (!value) {
      continue;
    }

    if (factor === "tag" || factor === "tagsearch") {
      parsed.tag = value;
    } else if (factor === "text" || factor === "content" || factor === "contentsearch" || factor === "search") {
      parsed.text = value;
    } else if (factor === "visibility") {
      const visibility = value.toUpperCase();
      if (VALID_VISIBILITIES.has(visibility as Visibility)) {
        parsed.visibility = visibility as Visibility;
      }
    } else if (factor === "type") {
      parsed.type = normalizeMemoType(value);
    } else if (factor === "property.haslink") {
      parsed.type = "LINKED";
    } else if (factor === "property.hastasklist") {
      parsed.type = "TODO";
    } else if (factor === "property.hascode") {
      parsed.type = "CODE";
    } else if (factor === "displaytime" || factor === "display_time" || factor === "date" || factor === "day") {
      const from = getDateStampByDate(value);
      parsed.duration = { from, to: from + DAILY_TIMESTAMP };
    }
  }

  return parsed;
};

const HomeSidebar = () => {
  const t = useTranslate();
  const filterStore = useFilterStore();
  const memoStore = useMemoStore();
  const tagStore = useTagStore();
  const userStore = useUserStore();

  const memos = memoStore.state.memos.filter((m) => m.creatorUsername === userStore.getCurrentUsername() && m.rowStatus === "NORMAL");
  const tagsText = tagStore.state.tags;
  const [tags, setTags] = useState<string[]>([]);
  const [shortcuts, setShortcuts] = useState<Shortcut[]>([]);
  const [selectedShortcutId, setSelectedShortcutId] = useState<ShortcutId | undefined>();

  const fetchShortcuts = async () => {
    try {
      const { data } = await api.getShortcutList();
      setShortcuts(data.filter((shortcut) => shortcut.rowStatus === "NORMAL"));
    } catch (error: any) {
      console.error(error);
      toast.error(error.response?.data?.message ?? "Failed to fetch shortcuts");
    }
  };

  useEffect(() => {
    tagStore.fetchTags();
    if (!userStore.isVisitorMode()) {
      fetchShortcuts();
    }
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

  const handleTodoChip = () => {
    filterStore.setMemoTypeFilter(activeType === "TODO" ? undefined : "TODO");
  };

  const handleCodeChip = () => {
    filterStore.setMemoTypeFilter(activeType === "CODE" ? undefined : "CODE");
  };

  const handleLinkChip = () => {
    filterStore.setMemoTypeFilter(activeType === "LINKED" ? undefined : "LINKED");
  };

  const handleTagChipClick = (tag: string) => {
    const current = filterStore.state.tag;
    filterStore.setTagFilter(current === tag ? undefined : tag);
    setSelectedShortcutId(undefined);
  };

  const activeTagFilter = filterStore.state.tag;

  const handleCreateShortcut = () => {
    showCreateShortcutDialog(undefined, fetchShortcuts);
  };

  const handleEditShortcut = (event: React.MouseEvent, shortcut: Shortcut) => {
    event.stopPropagation();
    showCreateShortcutDialog(shortcut, fetchShortcuts);
  };

  const handleDeleteShortcut = (event: React.MouseEvent, shortcut: Shortcut) => {
    event.stopPropagation();
    showCommonDialog({
      dialogName: "delete-shortcut-dialog",
      title: "Delete shortcut",
      content: `Are you sure you want to delete "${shortcut.title}"?`,
      style: "warning",
      confirmBtnText: t("common.delete"),
      onConfirm: async () => {
        try {
          await api.deleteShortcut(shortcut.id);
          if (selectedShortcutId === shortcut.id) {
            setSelectedShortcutId(undefined);
          }
          await fetchShortcuts();
          toast.success("Delete shortcut successfully");
        } catch (error: any) {
          console.error(error);
          toast.error(error.response?.data?.message ?? "Failed to delete shortcut");
        }
      },
    });
  };

  const handleShortcutClick = (shortcut: Shortcut) => {
    if (selectedShortcutId === shortcut.id) {
      filterStore.clearFilter();
      setSelectedShortcutId(undefined);
      return;
    }

    const nextFilter = parseShortcutPayload(shortcut.payload);
    filterStore.clearFilter();
    filterStore.setFilter(nextFilter);
    setSelectedShortcutId(shortcut.id);
  };

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
          <button
            onClick={handleTodoChip}
            className={`flex items-center gap-1 px-2.5 py-1 rounded-md border text-xs font-medium transition-colors ${
              activeType === "TODO"
                ? "bg-blue-100 border-blue-300 text-blue-700 dark:bg-blue-900 dark:border-blue-600 dark:text-blue-200"
                : "bg-white border-gray-200 text-gray-600 dark:bg-zinc-800 dark:border-zinc-600 dark:text-gray-300 hover:border-gray-300"
            }`}
          >
            <Icon.CheckSquare className="w-3.5 h-auto" />
            To-do {todoDone}/{todoTotal}
          </button>
          <button
            onClick={handleCodeChip}
            className={`flex items-center gap-1 px-2.5 py-1 rounded-md border text-xs font-medium transition-colors ${
              activeType === "CODE"
                ? "bg-blue-100 border-blue-300 text-blue-700 dark:bg-blue-900 dark:border-blue-600 dark:text-blue-200"
                : "bg-white border-gray-200 text-gray-600 dark:bg-zinc-800 dark:border-zinc-600 dark:text-gray-300 hover:border-gray-300"
            }`}
          >
            <Icon.Code2 className="w-3.5 h-auto" />
            Code {codeCount}
          </button>
        </div>
      )}

      {/* Shortcuts */}
      {!userStore.isVisitorMode() && (
        <div className="w-full px-3 mb-1">
          <div className="flex items-center justify-between py-1">
            <span className="text-sm font-medium text-gray-600 dark:text-gray-300">Shortcuts</span>
            <button
              onClick={handleCreateShortcut}
              className="flex items-center justify-center w-6 h-6 rounded hover:bg-gray-200 dark:hover:bg-zinc-700 text-gray-400 cursor-pointer"
            >
              <Icon.Plus className="w-4 h-4" />
            </button>
          </div>
          <div className="flex flex-col gap-1">
            {shortcuts.map((shortcut) => {
              const selected = selectedShortcutId === shortcut.id;
              return (
                <button
                  key={shortcut.id}
                  onClick={() => handleShortcutClick(shortcut)}
                  className={`group w-full flex items-center justify-between gap-2 px-2 py-1 rounded-md text-sm transition-colors ${
                    selected
                      ? "bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200"
                      : "text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-zinc-800"
                  }`}
                >
                  <span className="truncate text-left">{shortcut.title}</span>
                  <span className="shrink-0 flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    <Icon.Edit3 className="w-3.5 h-3.5" onClick={(event) => handleEditShortcut(event, shortcut)} />
                    <Icon.Trash className="w-3.5 h-3.5" onClick={(event) => handleDeleteShortcut(event, shortcut)} />
                  </span>
                </button>
              );
            })}
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
