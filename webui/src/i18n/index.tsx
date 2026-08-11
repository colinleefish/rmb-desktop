import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import {
  translations,
  type Lang,
  type Translation,
} from "./translations";
import { isLang, detectBrowserLang } from "./languages";

export { SUPPORTED_LANGUAGES, isLang, DEFAULT_LANG, detectBrowserLang } from "./languages";

const STORAGE_KEY = "rmb.lang";

type I18nContextValue = {
  lang: Lang;
  t: Translation;
  setLang: (lang: Lang) => void;
};

const I18nContext = createContext<I18nContextValue | null>(null);

function readLang(): Lang {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored && isLang(stored)) return stored;
  return detectBrowserLang();
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<Lang>(readLang);

  const setLang = useCallback((next: Lang) => {
    localStorage.setItem(STORAGE_KEY, next);
    setLangState(next);
    document.documentElement.lang = next;
  }, []);

  const value = useMemo<I18nContextValue>(
    () => ({
      lang,
      t: translations[lang],
      setLang,
    }),
    [lang, setLang],
  );

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error("useI18n must be used within I18nProvider");
  return ctx;
}
