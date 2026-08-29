export function AdminBrand({ subtitle }: { subtitle: string }) {
  return (
    <div className="flex items-center gap-3.5 tracking-[0.12em]">
      <span className="border-admin-accent-border text-admin-accent-bright rounded-admin-input bg-admin-accent-soft grid h-10.5 w-10.5 place-items-center border font-mono font-extrabold">
        7S
      </span>
      <div className="grid gap-0.5">
        <strong className="text-admin-ink-strong font-bold tracking-[0.12em]">
          CONTROL ROOM
        </strong>
        <small className="text-admin-muted-subtle tracking-normal">
          {subtitle}
        </small>
      </div>
    </div>
  )
}
