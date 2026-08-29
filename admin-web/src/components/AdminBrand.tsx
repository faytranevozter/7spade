export function AdminBrand({ subtitle }: { subtitle: string }) {
  return (
    <div className="flex items-center gap-3.5 tracking-[0.12em]">
      <span className="border-admin-control-accent text-admin-control-accent grid h-10.5 w-10.5 place-items-center border font-mono font-extrabold">
        7S
      </span>
      <div className="grid gap-0.5">
        <strong className="font-bold tracking-[0.12em] text-white">
          CONTROL ROOM
        </strong>
        <small className="text-admin-control-muted-subtle tracking-normal">
          {subtitle}
        </small>
      </div>
    </div>
  )
}
