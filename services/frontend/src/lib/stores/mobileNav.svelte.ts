class MobileNavStore {
  open = $state(false);
  toggle() { this.open = !this.open; }
  close() { this.open = false; }
}
export const mobileNav = new MobileNavStore();
