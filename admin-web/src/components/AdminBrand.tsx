export function AdminBrand({ subtitle }: { subtitle: string }) {
  return (
    <div className="brand">
      <span className="mark">7S</span>
      <div>
        <strong>CONTROL ROOM</strong>
        <small>{subtitle}</small>
      </div>
    </div>
  )
}
