export function toDisplayDate(iso: string | null) {
  return iso ? iso.split("-").reverse().join("/") : "";
}

export function toISODate(value: string) {
  const parts = /^(\d{2})\/(\d{2})\/(\d{4})$/.exec(value);
  if (!parts || parts[3] === "0000") return null;
  const iso = `${parts[3]}-${parts[2]}-${parts[1]}`;
  const date = new Date(`${iso}T00:00:00Z`);
  return Number.isFinite(date.getTime()) &&
    date.toISOString().slice(0, 10) === iso
    ? iso
    : null;
}
