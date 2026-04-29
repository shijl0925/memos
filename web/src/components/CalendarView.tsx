import { useEffect, useState } from "react";
import { getMemoStats } from "@/helpers/api";
import { DAILY_TIMESTAMP } from "@/helpers/consts";
import { getDateStampByDate } from "@/helpers/datetime";
import { useFilterStore, useMemoStore, useUserStore } from "@/store/module";
import { useTranslate } from "@/utils/i18n";
import Icon from "./Icon";

const WEEK_DAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

const getCalendarDateString = (timestamp: number) => {
  const date = new Date(timestamp);
  const year = date.getFullYear();
  const month = `${date.getMonth() + 1}`.padStart(2, "0");
  const day = `${date.getDate()}`.padStart(2, "0");
  return `${year}-${month}-${day}`;
};

const getTooltipPositionClass = (dayIndex: number) => {
  if (dayIndex <= 1) {
    return "left-0";
  }
  if (dayIndex >= 5) {
    return "right-0";
  }
  return "left-1/2 -translate-x-1/2";
};

const CalendarView = () => {
  const t = useTranslate();
  const filterStore = useFilterStore();
  const memoStore = useMemoStore();
  const userStore = useUserStore();
  const today = new Date();
  const [viewYear, setViewYear] = useState(today.getFullYear());
  const [viewMonth, setViewMonth] = useState(today.getMonth()); // 0-indexed
  const [memoCreatedDateCounts, setMemoCreatedDateCounts] = useState<Map<number, number>>(new Map());

  const todayStamp = getDateStampByDate(today);
  const currentUsername = userStore.getCurrentUsername();

  useEffect(() => {
    getMemoStats(currentUsername)
      .then(({ data }) => {
        const dateCounts = new Map<number, number>();
        for (const createdTs of data) {
          const dateStamp = getDateStampByDate(createdTs * 1000);
          dateCounts.set(dateStamp, (dateCounts.get(dateStamp) ?? 0) + 1);
        }
        setMemoCreatedDateCounts(dateCounts);
      })
      .catch((error) => {
        console.error("Failed to load memo statistics", error);
      });
  }, [memoStore.state.memos.length, currentUsername]);

  // Build calendar grid
  const firstDayOfMonth = new Date(viewYear, viewMonth, 1);
  const firstWeekDay = firstDayOfMonth.getDay(); // 0 = Sun
  const daysInMonth = new Date(viewYear, viewMonth + 1, 0).getDate();

  // Days from previous month to fill in leading cells
  const prevMonthDays = new Date(viewYear, viewMonth, 0).getDate();

  type CalendarCell = { day: number; month: "prev" | "current" | "next"; timestamp: number };
  const cells: CalendarCell[] = [];

  // Leading days from previous month
  for (let i = firstWeekDay - 1; i >= 0; i--) {
    const d = prevMonthDays - i;
    const date = new Date(viewYear, viewMonth - 1, d);
    cells.push({ day: d, month: "prev", timestamp: getDateStampByDate(date) });
  }

  // Current month days
  for (let d = 1; d <= daysInMonth; d++) {
    const date = new Date(viewYear, viewMonth, d);
    cells.push({ day: d, month: "current", timestamp: getDateStampByDate(date) });
  }

  // Trailing days to fill out last row (up to 42 cells = 6 rows)
  const trailing = 42 - cells.length;
  for (let d = 1; d <= trailing; d++) {
    const date = new Date(viewYear, viewMonth + 1, d);
    cells.push({ day: d, month: "next", timestamp: getDateStampByDate(date) });
  }

  const handlePrevMonth = () => {
    if (viewMonth === 0) {
      setViewMonth(11);
      setViewYear((y) => y - 1);
    } else {
      setViewMonth((m) => m - 1);
    }
  };

  const handleNextMonth = () => {
    if (viewMonth === 11) {
      setViewMonth(0);
      setViewYear((y) => y + 1);
    } else {
      setViewMonth((m) => m + 1);
    }
  };

  const handleDayClick = (cell: CalendarCell) => {
    if (cell.month !== "current") return;
    const currentFilter = filterStore.getState();
    if (currentFilter.duration?.from === cell.timestamp) {
      filterStore.setFromAndToFilter();
    } else {
      filterStore.setFromAndToFilter(cell.timestamp, cell.timestamp + DAILY_TIMESTAMP);
    }
  };

  const selectedFrom = filterStore.state.duration?.from;

  const monthName = new Date(viewYear, viewMonth, 1).toLocaleString("default", { month: "long" });

  return (
    <div className="relative w-full overflow-visible px-3 py-2 select-none">
      {/* Month header */}
      <div className="flex items-center justify-between mb-2">
        <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">
          {monthName} {viewYear}
        </span>
        <div className="flex items-center gap-1">
          <button
            onClick={handlePrevMonth}
            className="p-1 rounded hover:bg-gray-200 dark:hover:bg-zinc-600 text-gray-500 dark:text-gray-400"
          >
            <Icon.ChevronLeft className="w-4 h-4" />
          </button>
          <button
            onClick={handleNextMonth}
            className="p-1 rounded hover:bg-gray-200 dark:hover:bg-zinc-600 text-gray-500 dark:text-gray-400"
          >
            <Icon.ChevronRight className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Week-day headers */}
      <div className="grid grid-cols-7 mb-1">
        {WEEK_DAYS.map((d) => (
          <div key={d} className="text-center text-xs text-gray-400 dark:text-zinc-500 font-medium py-1">
            {d}
          </div>
        ))}
      </div>

      {/* Day cells */}
      <div className="relative grid grid-cols-7 overflow-visible">
        {cells.map((cell, idx) => {
          const isToday = cell.month === "current" && cell.timestamp === todayStamp;
          const isSelected = cell.month === "current" && cell.timestamp === selectedFrom;
          const isCurrentMonth = cell.month === "current";
          const memoCreatedCount = isCurrentMonth ? memoCreatedDateCounts.get(cell.timestamp) ?? 0 : 0;
          const hasMemoCreated = memoCreatedCount > 0;
          const memoTooltip = hasMemoCreated
            ? t(memoCreatedCount === 1 ? "heatmap.memo-on" : "heatmap.memos-on", {
                amount: memoCreatedCount,
                date: getCalendarDateString(cell.timestamp),
              })
            : "";
          const memoTooltipId = hasMemoCreated ? `calendar-memo-tooltip-${cell.timestamp}` : undefined;
          const tooltipPositionClass = getTooltipPositionClass(idx % 7);

          return (
            <div
              key={idx}
              role={isCurrentMonth ? "button" : undefined}
              tabIndex={isCurrentMonth ? 0 : undefined}
              className={`group relative flex items-center justify-center overflow-visible py-0.5 hover:z-50 focus:z-50 focus:outline-none ${
                isCurrentMonth ? "cursor-pointer" : "cursor-default"
              }`}
              onClick={() => handleDayClick(cell)}
              onKeyDown={(event) => {
                if (!isCurrentMonth || (event.key !== "Enter" && event.key !== " ")) {
                  return;
                }
                event.preventDefault();
                handleDayClick(cell);
              }}
            >
              <span
                aria-describedby={memoTooltipId}
                className={`
                  relative w-7 h-7 flex items-center justify-center rounded-full text-xs
                  ${!isCurrentMonth ? "text-gray-300 dark:text-zinc-600" : "text-gray-700 dark:text-gray-200"}
                  ${isToday && !isSelected ? "bg-red-700 text-white font-bold" : ""}
                  ${isSelected ? "bg-blue-500 text-white font-bold" : ""}
                  ${isCurrentMonth ? "group-focus-visible:ring-2 group-focus-visible:ring-blue-400 group-focus-visible:ring-offset-1 group-focus-visible:ring-offset-zinc-50 dark:group-focus-visible:ring-offset-zinc-900" : ""}
                  ${
                    isCurrentMonth && !isToday && !isSelected
                      ? "group-hover:bg-gray-200 group-focus:bg-gray-200 dark:group-hover:bg-zinc-600 dark:group-focus:bg-zinc-600"
                      : ""
                  }
                `}
              >
                {cell.day}
                {hasMemoCreated && (
                  <>
                    <span className="absolute top-0 right-0.5 w-1.5 h-1.5 rounded-full bg-blue-500 ring-1 ring-white dark:ring-zinc-800" />
                    <span
                      id={memoTooltipId}
                      role="tooltip"
                      className={`pointer-events-none absolute bottom-full z-50 mb-1 whitespace-nowrap rounded bg-gray-800 px-2 py-1 text-xs font-normal text-white opacity-0 transition-opacity group-hover:opacity-100 group-focus:opacity-100 ${tooltipPositionClass}`}
                    >
                      {memoTooltip}
                    </span>
                  </>
                )}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
};

export default CalendarView;
