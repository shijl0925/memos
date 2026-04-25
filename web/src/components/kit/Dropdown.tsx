import { ReactNode, useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import useToggle from "@/hooks/useToggle";
import Icon from "../Icon";

interface Props {
  trigger?: ReactNode;
  actions?: ReactNode;
  className?: string;
  actionsClassName?: string;
  positionClassName?: string;
}

const Dropdown: React.FC<Props> = (props: Props) => {
  const { trigger, actions, className, actionsClassName, positionClassName } = props;
  const [dropdownStatus, toggleDropdownStatus] = useToggle(false);
  const dropdownWrapperRef = useRef<HTMLDivElement>(null);
  const [panelStyle, setPanelStyle] = useState<React.CSSProperties>({});

  useLayoutEffect(() => {
    if (dropdownStatus && dropdownWrapperRef.current) {
      const rect = dropdownWrapperRef.current.getBoundingClientRect();
      const isRightAligned = positionClassName?.includes("right-0") ?? false;
      const marginTop = positionClassName?.includes("mt-2") ? 8 : 4;
      setPanelStyle({
        position: "fixed",
        top: rect.bottom + marginTop,
        ...(isRightAligned ? { right: window.innerWidth - rect.right } : { left: rect.left }),
        zIndex: 9999,
      });
    }
  }, [dropdownStatus, positionClassName]);

  useEffect(() => {
    if (dropdownStatus) {
      const handleClickOutside = (event: MouseEvent) => {
        if (!dropdownWrapperRef.current?.contains(event.target as Node)) {
          toggleDropdownStatus(false);
        }
      };
      window.addEventListener("click", handleClickOutside, {
        capture: true,
        once: true,
      });
    }
  }, [dropdownStatus]);

  const panel = (
    <div
      className={`w-auto flex flex-col justify-start items-start bg-white dark:bg-zinc-700 p-1 rounded-md shadow ${
        dropdownStatus ? "" : "!hidden"
      } ${actionsClassName ?? ""}`}
      style={panelStyle}
    >
      {actions}
    </div>
  );

  return (
    <div
      ref={dropdownWrapperRef}
      className={`relative flex flex-col justify-start items-start select-none ${className ?? ""}`}
      onClick={() => toggleDropdownStatus()}
    >
      {trigger ? (
        trigger
      ) : (
        <button className="flex flex-row justify-center items-center border dark:border-zinc-700 p-1 rounded shadow text-gray-600 dark:text-gray-200 cursor-pointer hover:opacity-80">
          <Icon.MoreHorizontal className="w-4 h-auto" />
        </button>
      )}
      {createPortal(panel, document.body)}
    </div>
  );
};

export default Dropdown;
