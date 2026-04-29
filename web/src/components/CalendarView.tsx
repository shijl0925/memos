import { useEffect, useState } from "react";
import { getMemoStats } from "@/helpers/api";
import { DAILY_TIMESTAMP } from "@/helpers/consts";
import { getDateStampByDate, getDateString } from "@/helpers/datetime";
import { useFilterStore, useMemoStore, useUserStore } from "@/store/module";
import Icon from "./Icon";

const WEEK_DAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

const CalendarView = () => {
  const filterStore = useFilterStore();
  const memoStore = useMemoStore();
  const userStore = useUserStore();
  const today = new Date();
  const [viewYear, setViewYear] = useState(today.getFullYear());
  const [viewMonth, setViewMonth] = useState(today.getMonth()); // 0-indexed
  const [memoCreatedDateStamps, setMemoCreatedDateStamps] = useState<Set<number>>(new Set());

  const todayStamp = getDateStampByDate(today);
  const memos = memoStore.state.memos;
  const memoAmount = memos.length;
  const currentUsername = userStore.getCurrentUsername();

  useEffect(() => {
    getMemoStats(currentUsername)
      .then(({ data }) => {
        setMemoCreatedDateStamps(new Set(data.map((createdTs) => getDateStampByDate(createdTs * 1000))));
      })
      .catch((error) => {
        console.error(error);
      });
  }, [memoAmount, currentUsername]);

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
    <div className="w-full px-3 py-2 select-none">
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
      <div className="grid grid-cols-7">
        {cells.map((cell, idx) => {
          const isToday = cell.month === "current" && cell.timestamp === todayStamp;
          const isSelected = cell.month === "current" && cell.timestamp === selectedFrom;
          const isCurrentMonth = cell.month === "current";
          const hasMemoCreated = isCurrentMonth && memoCreatedDateStamps.has(cell.timestamp);

          return (
            <div
              key={idx}
              className={`flex items-center justify-center py-0.5 ${isCurrentMonth ? "cursor-pointer" : "cursor-default"}`}
              onClick={() => handleDayClick(cell)}
            >
              <span
                className={`
                  relative w-7 h-7 flex items-center justify-center rounded-full text-xs
                  ${!isCurrentMonth ? "text-gray-300 dark:text-zinc-600" : "text-gray-700 dark:text-gray-200"}
                  ${isToday && !isSelected ? "bg-red-700 text-white font-bold" : ""}
                  ${isSelected ? "bg-blue-500 text-white font-bold" : ""}
                  ${isCurrentMonth && !isToday && !isSelected ? "hover:bg-gray-200 dark:hover:bg-zinc-600" : ""}
                `}
              >
                {cell.day}
                {hasMemoCreated && (
                  <>
                    <span className="sr-only">Memos created on {getDateString(cell.timestamp)}</span>
                    <span className="absolute top-1.5 right-1.5 w-1.5 h-1.5 rounded-full bg-blue-500 ring-1 ring-white dark:ring-zinc-800" />
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
