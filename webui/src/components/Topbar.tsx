import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { Search } from "lucide-react";
import { useLocation, useNavigate, useSearchParams } from "react-router-dom";
import { useI18n } from "../i18n";
import { getRouteMeta } from "../lib/shellRoutes";

export function Topbar() {
  const { t } = useI18n();
  const location = useLocation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const inputRef = useRef<HTMLInputElement>(null);
  const [query, setQuery] = useState(searchParams.get("q") ?? "");

  useEffect(() => {
    if (location.pathname.startsWith("/memories")) {
      setQuery(searchParams.get("q") ?? "");
    }
  }, [location.pathname, searchParams]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        inputRef.current?.focus();
        inputRef.current?.select();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const { title, subtitle } = useMemo(
    () => getRouteMeta(location.pathname, t),
    [location.pathname, t],
  );

  function submitSearch(e: FormEvent) {
    e.preventDefault();
    const q = query.trim();
    const params = new URLSearchParams(location.search);
    if (q) params.set("q", q);
    else params.delete("q");
    const qs = params.toString();
    if (location.pathname.startsWith("/memories")) {
      navigate({ pathname: location.pathname, search: qs ? `?${qs}` : "" });
      return;
    }
    navigate({ pathname: "/memories", search: q ? `?q=${encodeURIComponent(q)}` : "" });
  }

  return (
    <header className="flex h-14 shrink-0 items-center gap-6 border-b border-rmb-line bg-white px-8">
      <div className="flex min-w-0 flex-1 items-baseline gap-3">
        <h1 className="shrink-0 text-[15px] font-semibold text-rmb-dark">{title}</h1>
        {subtitle ? (
          <p className="hidden truncate text-xs text-rmb-muted md:block">{subtitle}</p>
        ) : null}
      </div>
      <form onSubmit={submitSearch} className="relative w-72 shrink-0">
        <Search
          className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-rmb-faint"
          strokeWidth={1.75}
        />
        <input
          ref={inputRef}
          type="search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t.shell.searchPlaceholder}
          className="h-8 w-full rounded-md border border-transparent bg-rmb-fill pl-8 pr-12 text-sm text-rmb-dark outline-none transition-colors placeholder:text-rmb-faint focus:border-rmb-accent focus:bg-white"
        />
        <kbd className="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 rounded border border-rmb-line bg-white px-1 font-sans text-[10px] leading-4 text-rmb-faint">
          ⌘K
        </kbd>
      </form>
    </header>
  );
}
