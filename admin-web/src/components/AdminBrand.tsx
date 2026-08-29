export function AdminBrand({ subtitle }: { subtitle: string }) {
  return (
    <div className="flex items-center gap-3.5 tracking-[0.12em]">
      <span className="w-[42px] h-[42px] grid place-items-center border border-[#4dd0b5] text-[#4dd0b5] font-mono font-extrabold">
        7S
      </span>
      <div className="grid gap-0.5">
        <strong className="font-bold text-white tracking-[0.12em]">
          CONTROL ROOM
        </strong>
        <small className="text-[#8493a5] tracking-normal">{subtitle}</small>
      </div>
    </div>
  )
}
