/**
 * DrawerCoordinator — singleton module that enforces drawer mutual exclusion
 * and coordinates Ambient-CTA visibility.
 *
 * Rules:
 * 1. Only one drawer may be open at a time.
 * 2. When any drawer opens, all Ambient-CTAs hide.
 * 3. When all drawers close, all Ambient-CTAs show.
 * 4. Drawers and CTAs register/deregister independently.
 */

export interface DrawerParticipant {
  readonly drawerId: string;
  closeDrawer(): void;
}

export interface AmbientCTA {
  setAmbientVisible(visible: boolean): void;
}

const drawers = new Map<string, DrawerParticipant>();
const ambientCTAs = new Set<AmbientCTA>();
const openDrawers = new Set<string>();

export function registerDrawer(participant: DrawerParticipant): () => void {
  drawers.set(participant.drawerId, participant);
  return () => {
    drawers.delete(participant.drawerId);
    openDrawers.delete(participant.drawerId);
  };
}

export function registerAmbientCTA(cta: AmbientCTA): () => void {
  ambientCTAs.add(cta);
  return () => {
    ambientCTAs.delete(cta);
  };
}

export function notifyDrawerOpened(drawerId: string): void {
  openDrawers.add(drawerId);

  for (const [id, participant] of drawers) {
    if (id !== drawerId) {
      participant.closeDrawer();
    }
  }

  for (const cta of ambientCTAs) {
    cta.setAmbientVisible(false);
  }
}

export function notifyDrawerClosed(drawerId: string): void {
  openDrawers.delete(drawerId);

  if (openDrawers.size === 0) {
    for (const cta of ambientCTAs) {
      cta.setAmbientVisible(true);
    }
  }
}

/** Reset all state — only for use in tests. */
export function resetForTesting(): void {
  drawers.clear();
  ambientCTAs.clear();
  openDrawers.clear();
}
