import { isEqual } from "lodash-es";
import { useEffect, useState } from "react";
import toast from "react-hot-toast";
import { cn } from "@/utils";

// Helper function to convert Date to local datetime string.
const toLocalDateTimeString = (date: Date | undefined): string => {
  return date?.toLocaleString() || "";
};

interface Props {
  value: Date | undefined;
  originalValue: Date | undefined;
  onChange: (date: Date) => void;
}

const DateTimeInput: React.FC<Props> = ({ value, originalValue, onChange }) => {
  const [inputValue, setInputValue] = useState(toLocalDateTimeString(value));

  useEffect(() => {
    setInputValue(toLocalDateTimeString(value));
  }, [value]);

  return (
    <input
      type="text"
      className={cn(
        "px-1 bg-transparent rounded text-xs transition-all",
        "border-transparent outline-none focus:border-gray-300 dark:focus:border-zinc-700",
        !isEqual(value, originalValue) && "border-gray-300 dark:border-zinc-700",
        "border",
      )}
      value={inputValue}
      onChange={(e) => setInputValue(e.target.value)}
      onBlur={(e) => {
        const rawValue = e.target.value.trim();
        if (rawValue) {
          const date = new Date(rawValue);
          if (!isNaN(date.getTime())) {
            onChange(date);
          } else {
            toast.error("Invalid datetime format. Use format: 2023-12-31T23:59:59");
            setInputValue(toLocalDateTimeString(value));
          }
        } else {
          setInputValue(toLocalDateTimeString(value));
        }
      }}
      placeholder="YYYY-MM-DDTHH:mm:ss"
    />
  );
};

export default DateTimeInput;
