/* eslint-disable no-undef */
const colors = require("tailwindcss/colors");
const themedColor = (name, hex) => {
  const rgb = [1, 3, 5].map((offset) => parseInt(hex.slice(offset, offset + 2), 16)).join(" ");
  return `rgb(var(--memos-${name}, ${rgb}) / <alpha-value>)`;
};

/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: ["class"],
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  prefix: "",
  theme: {
    extend: {
      colors: {
        ...Object.fromEntries(
          ["gray", "zinc"].map((family) => [
            family,
            Object.fromEntries(Object.entries(colors[family]).map(([shade, hex]) => [shade, themedColor(`${family}-${shade}`, hex)])),
          ]),
        ),
        primary: {
          DEFAULT: themedColor("primary", "#0d9488"), // Teal 600
          dark: themedColor("primary-dark", "#0f766e"), // Teal 700
          darker: themedColor("primary-darker", "#0d5a56"), // Teal 800
          lighter: themedColor("primary-lighter", "#14b8a6"), // Teal 500
        },
        success: {
          DEFAULT: "#16a34a", // Green 600
          dark: "#047857", // Green 700
          darker: "#03664a", // Green 800
        },
        danger: {
          DEFAULT: "#dc2626", // Red 600
          dark: "#b91c1c", // Red 700
          darker: "#991b1b", // Red 800
        },
        warning: {
          DEFAULT: "#ca8a04", // Yellow 600
          dark: "#b45309", // Yellow 700
          darker: "#92400e", // Yellow 800
        },
      },
      spacing: {
        128: "32rem",
      },
      zIndex: {
        1: "1",
        2: "2",
        20: "20",
        100: "100",
        1000: "1000",
        2000: "2000",
      },
      gridTemplateRows: {
        7: "repeat(7, minmax(0, 1fr))",
      },
    },
  },
  plugins: [require("tailwindcss-animate")],
};
