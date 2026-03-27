import type { LitElement } from 'lit';
import { state } from 'lit/decorators.js';
import {
  registerDrawer,
  notifyDrawerOpened,
  notifyDrawerClosed,
  type DrawerParticipant,
} from './drawer-coordinator.js';

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- Lit mixin constructor pattern requires any
type Constructor<T = LitElement> = abstract new (...args: any[]) => T;

export declare class DrawerMixinInterface {
  drawerOpen: boolean;
  readonly drawerId: string;
  openDrawer(): void;
  closeDrawer(): void;
  toggleDrawer(): void;
}

export function DrawerMixin<T extends Constructor>(Base: T) {
  abstract class DrawerMixinClass extends Base implements DrawerParticipant {
    @state()
    declare drawerOpen: boolean;

    abstract readonly drawerId: string;

    private _drawerCleanup: (() => void) | undefined;

    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- Lit mixin constructor pattern requires any
    constructor(...args: any[]) {
      super(...args);
      this.drawerOpen = false;
    }

    override connectedCallback(): void {
      super.connectedCallback();
      this._drawerCleanup = registerDrawer(this);
    }

    override disconnectedCallback(): void {
      super.disconnectedCallback();
      this._drawerCleanup?.();
      this._drawerCleanup = undefined;
    }

    openDrawer(): void {
      this.drawerOpen = true;
      notifyDrawerOpened(this.drawerId);
    }

    closeDrawer(): void {
      if (!this.drawerOpen) return;
      this.drawerOpen = false;
      notifyDrawerClosed(this.drawerId);
    }

    toggleDrawer(): void {
      if (this.drawerOpen) {
        this.closeDrawer();
      } else {
        this.openDrawer();
      }
    }
  }

  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- Required for Lit mixin pattern
  return DrawerMixinClass as unknown as Constructor<DrawerMixinInterface> & T;
}
