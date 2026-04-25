import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import type { MsgKey, Messages } from "./keys";
import { en } from "./en";
import { ja } from "./ja";

export type Lang = "en" | "ja";

const STORAGE_KEY = "shepherd.lang";

const messagesMap: Record<Lang, Messages> = { en, ja };

interface I18nContextValue {
  lang: Lang;
  setLang: (lang: Lang) => void;
  t: (key: MsgKey, params?: Record<string, string | number>) => string;
}

const I18nContext = createContext<I18nContextValue | null>(null);

function detectBrowserLang(fallback: Lang): Lang {
  if (typeof navigator === "undefined") return fallback;
  const nav = navigator.language || "";
  if (nav.toLowerCase().startsWith("ja")) return "ja";
  return "en";
}

function loadStoredLang(): Lang | null {
  if (typeof window === "undefined") return null;
  const stored = window.localStorage.getItem(STORAGE_KEY);
  if (stored === "en" || stored === "ja") return stored;
  return null;
}

function interpolate(
  template: string,
  params: Record<string, string | number>,
): string {
  return template.replace(/\{(\w+)\}/g, (_, key: string) => {
    if (!(key in params)) return `{${key}}`;
    return String(params[key]);
  });
}

export function I18nProvider({
  defaultLang = "en",
  children,
}: {
  defaultLang?: Lang;
  children: ReactNode;
}) {
  const [lang, setLangState] = useState<Lang>(
    () => loadStoredLang() ?? detectBrowserLang(defaultLang),
  );

  useEffect(() => {
    if (typeof window !== "undefined") {
      window.localStorage.setItem(STORAGE_KEY, lang);
      document.documentElement.setAttribute("lang", lang);
    }
  }, [lang]);

  const setLang = useCallback((next: Lang) => setLangState(next), []);

  const t = useCallback(
    (key: MsgKey, params?: Record<string, string | number>): string => {
      const msg =
        messagesMap[lang]?.[key] ?? messagesMap[defaultLang]?.[key] ?? key;
      return params ? interpolate(msg, params) : msg;
    },
    [lang, defaultLang],
  );

  const value = useMemo(() => ({ lang, setLang, t }), [lang, setLang, t]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useTranslation(): I18nContextValue {
  const ctx = useContext(I18nContext);
  if (!ctx) {
    throw new Error("useTranslation must be used within I18nProvider");
  }
  return ctx;
}
