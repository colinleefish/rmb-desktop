import { createContext } from "react";
import type { Lang, Translation } from "./translations";

export type I18nContextValue = {
  lang: Lang;
  t: Translation;
  setLang: (lang: Lang) => void;
};

export const I18nContext = createContext<I18nContextValue | null>(null);
