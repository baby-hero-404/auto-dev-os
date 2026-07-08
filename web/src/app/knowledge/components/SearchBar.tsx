import { Loader2, Search } from "lucide-react";

interface SearchBarProps {
  searchQuery: string;
  isSearching: boolean;
  selectedTier: string;
  onSearchChange: (val: string) => void;
  onSearchSubmit: (e: React.FormEvent) => void;
  onClearSearch: () => void;
  onSelectTier: (tier: string) => void;
}

export function SearchBar({
  searchQuery,
  isSearching,
  selectedTier,
  onSearchChange,
  onSearchSubmit,
  onClearSearch,
  onSelectTier,
}: SearchBarProps) {
  return (
    <div className="rounded-lg border border-stroke bg-panel p-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <form onSubmit={onSearchSubmit} className="flex-1 flex gap-2 max-w-lg">
        <div className="relative flex-1">
          <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-content-muted" />
          <input
            type="text"
            placeholder="Triple-Stream Search (BM25 + Vector + Graph)..."
            value={searchQuery}
            onChange={(e) => onSearchChange(e.target.value)}
            className="w-full rounded-md border border-stroke bg-slate-950 pl-9 pr-3 py-2 text-sm text-white focus:outline-none focus:border-brand-primary"
          />
        </div>
        <button
          type="submit"
          disabled={isSearching}
          className="rounded-md bg-slate-900 border border-stroke px-4 py-2 text-sm font-semibold text-white hover:bg-slate-800 transition cursor-pointer flex items-center gap-1.5"
        >
          {isSearching ? <Loader2 size={14} className="animate-spin" /> : "Search"}
        </button>
        {searchQuery && (
          <button
            type="button"
            onClick={onClearSearch}
            className="rounded-md border border-red-500/20 bg-red-950/20 px-3 py-2 text-sm font-semibold text-red-300 hover:bg-red-950/40 transition cursor-pointer"
          >
            Clear
          </button>
        )}
      </form>

      {!searchQuery && (
        <div className="flex gap-1.5 overflow-x-auto pb-1 sm:pb-0">
          {["", "working", "episodic", "semantic", "procedural"].map((tier) => (
            <button
              key={tier}
              onClick={() => onSelectTier(tier)}
              className={`rounded px-2.5 py-1 text-xs font-mono border transition cursor-pointer capitalize ${
                selectedTier === tier
                  ? "bg-brand-primary/15 border-brand-primary text-brand-primary"
                  : "bg-slate-950 border-stroke text-content-muted hover:text-slate-200"
              }`}
            >
              {tier || "All Tiers"}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
