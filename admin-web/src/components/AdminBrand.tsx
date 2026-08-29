export function AdminBrand({ subtitle }: { subtitle: string }) {
  return (
    <div className="flex items-center gap-3.5 tracking-[0.12em]">
      <span className="grid h-10.5 w-10.5 place-items-center border border-[#4dd0b5] font-mono font-extrabold text-[#4dd0b5]">
        7S
      </span>
      <div className="grid gap-0.5">
        <strong className="font-bold tracking-[0.12em] text-white">
          CONTROL ROOM
        </strong>
        <small className="tracking-normal text-[#8493a5]">{subtitle}</small>
      </div>
    </div>
  )
}
