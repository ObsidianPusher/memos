import { LucideIcon } from "lucide-react";
import React from "react";

interface SettingMenuItemProps {
  text: string;
  icon: LucideIcon;
  isSelected: boolean;
  onClick: () => void;
}

const SectionMenuItem: React.FC<SettingMenuItemProps> = ({ text, icon: IconComponent, isSelected, onClick }) => {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={isSelected}
      className={`w-auto max-w-full px-3 leading-8 flex flex-row justify-start items-center cursor-pointer rounded-lg select-none text-left transition hover:opacity-80 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 dark:focus:ring-offset-zinc-800 ${
        isSelected ? "bg-zinc-100 shadow dark:bg-zinc-900" : ""
      }`}
    >
      <IconComponent className="w-4 h-auto mr-2 opacity-80 shrink-0" />
      <span className="truncate">{text}</span>
    </button>
  );
};

export default SectionMenuItem;
