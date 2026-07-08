export function SkillsGuide() {
  return (
    <section className="mb-6 rounded-lg border border-stroke bg-surface/20 p-4">
      <div className="flex flex-col gap-2 md:flex-row md:items-start md:justify-between">
        <div className="max-w-2xl">
          <h3 className="text-sm font-semibold text-foreground">Setup guide</h3>
          <p className="mt-1 text-xs leading-relaxed text-content-muted">
            Connect one Git repository, then sync it so the catalog can read <span className="font-mono text-foreground">registry.json</span> or{" "}
            <span className="font-mono text-foreground">registry.min.json</span> at the repo root.
          </p>
        </div>
        <div className="text-xs text-content-muted">Waiting for repository connection</div>
      </div>

      <div className="mt-4 grid gap-3 md:grid-cols-3">
        <div className="rounded-md border border-stroke bg-background p-3">
          <div className="text-[10px] uppercase tracking-wider text-content-muted">Step 1</div>
          <div className="mt-1 text-sm font-semibold text-foreground">Paste a Git URL</div>
          <p className="mt-1 text-xs leading-relaxed text-content-muted">Use HTTPS, SSH, git+ssh, or file://. The input is validated before connect.</p>
        </div>
        <div className="rounded-md border border-stroke bg-background p-3">
          <div className="text-[10px] uppercase tracking-wider text-content-muted">Step 2</div>
          <div className="mt-1 text-sm font-semibold text-foreground">Sync the repository</div>
          <p className="mt-1 text-xs leading-relaxed text-content-muted">The repo root must expose a registry file so skills can be indexed safely.</p>
        </div>
        <div className="rounded-md border border-stroke bg-background p-3">
          <div className="text-[10px] uppercase tracking-wider text-content-muted">Step 3</div>
          <div className="mt-1 text-sm font-semibold text-foreground">Inspect catalog entries</div>
          <p className="mt-1 text-xs leading-relaxed text-content-muted">Select a skill to view metadata, source folders, and file contents.</p>
        </div>
      </div>

      <div className="mt-4 grid gap-3 lg:grid-cols-2">
        <div className="rounded-md border border-stroke bg-background p-3">
          <div className="text-[10px] uppercase tracking-wider text-content-muted">Repo validation</div>
          <p className="mt-1 text-xs leading-relaxed text-content-muted">
            A suitable skills repo must be a normal Git repository, not a manually edited UI form. After sync, Auto Code OS reads the root registry
            file, indexes the listed skills, and exposes only the synced source files in read-only mode.
          </p>
          <div className="mt-3 grid gap-2 text-xs text-content-muted">
            <div className="rounded-md border border-stroke bg-surface/30 px-3 py-2">Root manifest: <span className="font-mono text-foreground">registry.json</span> or <span className="font-mono text-foreground">registry.min.json</span></div>
            <div className="rounded-md border border-stroke bg-surface/30 px-3 py-2">Catalog source: <span className="font-mono text-foreground">skills</span> entries in the manifest</div>
            <div className="rounded-md border border-stroke bg-surface/30 px-3 py-2">Editing rule: update the Git repo, then click Sync again</div>
          </div>
        </div>
        <div className="rounded-md border border-stroke bg-background p-3">
          <div className="text-[10px] uppercase tracking-wider text-content-muted">Valid URL examples</div>
          <div className="mt-2 grid gap-2 text-xs text-content-muted">
            <div className="rounded-md border border-stroke bg-surface/30 px-3 py-2 font-mono text-foreground">https://github.com/org/repo.git</div>
            <div className="rounded-md border border-stroke bg-surface/30 px-3 py-2 font-mono text-foreground">git@github.com:org/repo.git</div>
            <div className="rounded-md border border-stroke bg-surface/30 px-3 py-2 font-mono text-foreground">file:///tmp/local-skill-repo</div>
          </div>
        </div>
      </div>
    </section>
  );
}
