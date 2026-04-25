/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  darkMode: ["class", '[data-theme="dark"]'],
  theme: {
    extend: {
      colors: {
        brand: {
          DEFAULT: "var(--brand)",
          ink: "var(--brand-ink)",
          soft: "var(--brand-soft)",
          soft2: "var(--brand-soft-2)",
        },
        bg: {
          DEFAULT: "var(--bg)",
          elev: "var(--bg-elev)",
          sunken: "var(--bg-sunken)",
        },
        line: {
          DEFAULT: "var(--line)",
          strong: "var(--line-strong)",
        },
        ink: {
          1: "var(--ink-1)",
          2: "var(--ink-2)",
          3: "var(--ink-3)",
          4: "var(--ink-4)",
          5: "var(--ink-5)",
        },
        success: { DEFAULT: "var(--success)", soft: "var(--success-soft)" },
        warn: { DEFAULT: "var(--warn)", soft: "var(--warn-soft)" },
        danger: { DEFAULT: "var(--danger)", soft: "var(--danger-soft)" },
        info: { DEFAULT: "var(--info)", soft: "var(--info-soft)" },
      },
      fontFamily: {
        sans: [
          "Inter",
          "-apple-system",
          "BlinkMacSystemFont",
          "Segoe UI",
          "system-ui",
          "sans-serif",
        ],
        mono: [
          "ui-monospace",
          "JetBrains Mono",
          "SF Mono",
          "Menlo",
          "monospace",
        ],
      },
      fontSize: {
        "2xs": ["10px", "1.2"],
        meta: ["12px", "1.45"],
      },
      borderRadius: {
        1: "4px",
        2: "6px",
        3: "8px",
        4: "10px",
        5: "14px",
      },
      boxShadow: {
        1: "0 1px 0 rgba(28,24,20,.04), 0 1px 2px rgba(28,24,20,.04)",
        2: "0 1px 2px rgba(28,24,20,.05), 0 4px 12px rgba(28,24,20,.06)",
        3: "0 8px 28px rgba(28,24,20,.10), 0 2px 6px rgba(28,24,20,.06)",
        pop: "0 12px 40px rgba(28,24,20,.16), 0 0 0 1px rgba(28,24,20,.06)",
      },
    },
  },
  plugins: [],
};
