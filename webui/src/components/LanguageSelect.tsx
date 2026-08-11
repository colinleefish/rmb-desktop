import { SUPPORTED_LANGUAGES } from "../i18n";
import type { Lang } from "../i18n/translations";
import { SelectMenu } from "./SelectMenu";

export function LanguageSelect({
  value,
  onChange,
  id = "language-select",
  labelId,
}: {
  value: Lang;
  onChange: (lang: Lang) => void;
  id?: string;
  labelId?: string;
}) {
  const options = SUPPORTED_LANGUAGES.map((entry) => ({
    value: entry.id,
    label: entry.nativeLabel,
  }));

  return (
    <SelectMenu
      id={id}
      labelId={labelId}
      value={value}
      onChange={onChange}
      options={options}
    />
  );
}
