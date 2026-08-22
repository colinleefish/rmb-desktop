import { useCallback, useMemo, useState, type ReactNode } from "react";
import { translations, type Lang } from "./translations";
import { isLang, detectBrowserLang } from "./languages";
import { I18nContext, type I18nContextValue } from "./context";

const STORAGE_KEY = "rmb.lang";

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
