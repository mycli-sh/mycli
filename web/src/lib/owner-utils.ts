export function isSystemOwner(owner: string | undefined | null): boolean {
  if (!owner) return true;
  return owner.toLowerCase() === "system";
}
