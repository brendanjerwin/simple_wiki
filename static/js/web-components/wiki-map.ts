import { css, html, LitElement } from 'lit';
import { property, state } from 'lit/decorators.js';
import { styleMap } from 'lit/directives/style-map.js';
import { createClient, type Client } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { Leaflet as L } from './leaflet-accessor.js';
import { getGrpcWebTransport } from './grpc-transport.js';
import {
  GetMapRequestSchema,
  GetTrackGeometryRequestSchema,
  AddTrackRequestSchema,
  MapService,
  type Map as WikiMapMessage,
  type MapCircle,
  type MapMarker,
  type MapPolygon,
  type MapTrack,
  type TileLayer,
} from '../gen/api/v1/map_pb.js';
import {
  FileStorageService,
  UploadFileRequestSchema,
} from '../gen/api/v1/file_storage_pb.js';
import { showToast } from './toast-message.js';
import { ChatMarkdownRenderer } from './chat-markdown-renderer.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
import { foundationCSS } from './shared-styles.js';
import './error-display.js';

export interface WikiMapRenderer {
  render(
    container: HTMLElement,
    map: WikiMapMessage,
    popupRenderer: PopupRenderer,
    client: Client<typeof MapService>,
    page: string,
    mapName: string,
    isIntersected: boolean
  ): void;
  loadTracks(): void;
  destroy(): void;
}

export interface PopupRenderer {
  render(markdown: string): Promise<string>;
}

export type WikiMapRendererFactory = () => WikiMapRenderer;

function escapeHtml(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}

// LeafletTagControl is built lazily because `L.Control` is undefined in the
// JSDOM test environment (Leaflet's DOM side effects don't initialize there).
// Defining the class at module load would throw "Class extends value
// undefined". We resolve L.Control at construction time instead.
interface TagControl extends L.Control {
  updateTags(): void;
}

let LeafletTagControlCtor: (new (renderer: LeafletWikiMapRenderer, options?: L.ControlOptions) => TagControl) | null = null;

function getLeafletTagControlCtor(): (new (renderer: LeafletWikiMapRenderer, options?: L.ControlOptions) => TagControl) {
  if (LeafletTagControlCtor !== null) return LeafletTagControlCtor;
  LeafletTagControlCtor = class LeafletTagControl extends L.Control {
    private _listSection: HTMLElement | null = null;
    private _renderer: LeafletWikiMapRenderer;

    constructor(renderer: LeafletWikiMapRenderer, options?: L.ControlOptions) {
      super(options || { position: 'topright' });
      this._renderer = renderer;
    }

    override onAdd(_map: L.Map): HTMLElement {  // eslint-disable-line @typescript-eslint/no-unused-vars -- Leaflet Control API requires this signature
      const container = L.DomUtil.create('div', 'leaflet-control-layers leaflet-control-layers-collapsed');

      const toggle = L.DomUtil.create('a', 'leaflet-control-layers-toggle', container);
      toggle.href = '#';
      toggle.title = 'Tags';

      const section = L.DomUtil.create('section', 'leaflet-control-layers-list', container);
      this._listSection = section;

      L.DomEvent.on(container, 'mouseenter', () => {
        L.DomUtil.removeClass(container, 'leaflet-control-layers-collapsed');
        L.DomUtil.addClass(container, 'leaflet-control-layers-expanded');
      });
      L.DomEvent.on(container, 'mouseleave', () => {
        L.DomUtil.removeClass(container, 'leaflet-control-layers-expanded');
        L.DomUtil.addClass(container, 'leaflet-control-layers-collapsed');
      });

      L.DomEvent.disableClickPropagation(container);
      L.DomEvent.disableScrollPropagation(container);

      this.updateTags();

      return container;
    }

    updateTags(): void {
      if (!this._listSection) return;
      this._listSection.innerHTML = '';

      const allTags = Array.from(this._renderer.allKnownTags).sort();
      const sortedTags = allTags.filter(t => t !== 'untagged');
      if (allTags.includes('untagged')) {
        sortedTags.push('untagged');
      }

      const form = L.DomUtil.create('form', '', this._listSection);
      const title = L.DomUtil.create('div', '', form);
      title.style.fontWeight = 'bold';
      title.style.marginBottom = '4px';
      title.innerText = 'Tags';

      for (const tag of sortedTags) {
        const label = L.DomUtil.create('label', '', form);
        label.style.display = 'flex';
        label.style.alignItems = 'center';
        label.style.gap = '6px';
        label.style.cursor = 'pointer';
        label.style.wordBreak = 'break-word';
        label.style.marginBottom = '4px';

        const checkbox = L.DomUtil.create('input', '', label) as HTMLInputElement;
        checkbox.type = 'checkbox';
        checkbox.value = tag;

        checkbox.checked = this._renderer.checkedTags.has(tag);

        L.DomEvent.on(checkbox, 'change', () => {
          if (checkbox.checked) {
            this._renderer.checkedTags.add(tag);
          } else {
            this._renderer.checkedTags.delete(tag);
          }
          this._renderer.filterLayers();
        });

        const span = L.DomUtil.create('span', '', label);
        span.innerText = ' ' + tag;
      }

      if (this._renderer.failedTracks.size > 0) {
        const errorDiv = L.DomUtil.create('div', '', form);
        errorDiv.style.borderTop = '1px solid #d0d7de';
        errorDiv.style.marginTop = '6px';
        errorDiv.style.paddingTop = '6px';
        errorDiv.style.color = '#cf222e';
        errorDiv.style.fontSize = '0.85rem';

        const errorTitle = L.DomUtil.create('div', '', errorDiv);
        errorTitle.style.fontWeight = 'bold';
        errorTitle.style.marginBottom = '2px';
        errorTitle.innerText = 'Errors';

        for (const trackLabel of this._renderer.failedTracks) {
          const item = L.DomUtil.create('div', '', errorDiv);
          item.style.display = 'flex';
          item.style.alignItems = 'center';
          item.style.gap = '4px';
          item.innerText = `⚠️ ${trackLabel} failed`;
        }
      }
    }
  };
  return LeafletTagControlCtor;
}

export class LeafletWikiMapRenderer implements WikiMapRenderer {
  private map: L.Map | null = null;
  public overlays: { layer: L.Layer; tags: string[] }[] = [];
  public checkedTags: Set<string> = new Set<string>();
  public allKnownTags: Set<string> = new Set<string>();
  public failedTracks: Set<string> = new Set<string>();
  public tagControl: TagControl | null = null;

  private tagsInitialized = false;
  private mapMessage: WikiMapMessage | null = null;
  private client: Client<typeof MapService> | null = null;
  private page: string = '';
  private mapName: string = '';
  private popupRenderer: PopupRenderer | null = null;
  private tracksToLoad: MapTrack[] = [];
  private tracksLoaded = false;

  render(
    container: HTMLElement,
    mapMessage: WikiMapMessage,
    popupRenderer: PopupRenderer,
    client: Client<typeof MapService>,
    page: string,
    mapName: string,
    isIntersected: boolean
  ): void {
    this.destroy();
    this.mapMessage = mapMessage;
    this.map = L.map(container, {
      dragging: true,
      zoomControl: true,
      scrollWheelZoom: true,
      touchZoom: true,
      preferCanvas: false,
    });

    const view = mapMessage.view;
    const markerPoints = mapMessage.markers.map(marker => marker.position).filter(point => point != null);
    const firstPoint = markerPoints[0];
    const center: L.LatLngExpression = view?.center
      ? [view.center.lat, view.center.lon]
      : firstPoint
        ? [firstPoint.lat, firstPoint.lon]
        : [0, 0];
    this.map.setView(center, view?.zoom ?? 2);

    const tileLayer = selectedTileLayer(mapMessage.style?.availableTileLayers ?? [], mapMessage.style?.tileLayerId);
    if (tileLayer) {
      L.tileLayer(tileLayer.urlTemplate, {
        attribution: tileLayer.attributionHtml,
        maxZoom: 19,
      }).addTo(this.map);
    }

    this.overlays = [];

    const currentTags = new Set<string>();
    let hasUntagged = false;
    for (const marker of mapMessage.markers ?? []) {
      if (marker.tags && marker.tags.length > 0) {
        for (const tag of marker.tags) currentTags.add(tag);
      } else {
        hasUntagged = true;
      }
    }
    for (const polygon of mapMessage.polygons ?? []) {
      if (polygon.tags && polygon.tags.length > 0) {
        for (const tag of polygon.tags) currentTags.add(tag);
      } else {
        hasUntagged = true;
      }
    }
    for (const circle of mapMessage.circles ?? []) {
      if (circle.tags && circle.tags.length > 0) {
        for (const tag of circle.tags) currentTags.add(tag);
      } else {
        hasUntagged = true;
      }
    }
    for (const track of mapMessage.tracks ?? []) {
      if (track.tags && track.tags.length > 0) {
        for (const tag of track.tags) currentTags.add(tag);
      } else {
        hasUntagged = true;
      }
    }

    const oldCheckedTags = new Set(this.checkedTags);
    const newCheckedTags = new Set<string>();

    if (!this.tagsInitialized) {
      for (const tag of currentTags) {
        newCheckedTags.add(tag);
      }
      if (hasUntagged) {
        newCheckedTags.add('untagged');
      }
      this.tagsInitialized = true;
    } else {
      for (const tag of currentTags) {
        if (!this.allKnownTags.has(tag)) {
          newCheckedTags.add(tag);
        } else if (oldCheckedTags.has(tag)) {
          newCheckedTags.add(tag);
        }
      }
      if (hasUntagged) {
        if (!this.allKnownTags.has('untagged')) {
          newCheckedTags.add('untagged');
        } else if (oldCheckedTags.has('untagged')) {
          newCheckedTags.add('untagged');
        }
      }
    }

    this.allKnownTags = new Set(currentTags);
    if (hasUntagged) {
      this.allKnownTags.add('untagged');
    }
    this.checkedTags = newCheckedTags;

    for (const marker of mapMessage.markers) {
      this.renderMarker(marker, popupRenderer);
    }
    for (const polygon of mapMessage.polygons) {
      this.renderPolygon(polygon, popupRenderer);
    }
    for (const circle of mapMessage.circles) {
      this.renderCircle(circle, popupRenderer);
    }

    this.client = client;
    this.page = page;
    this.mapName = mapName;
    this.popupRenderer = popupRenderer;
    this.tracksToLoad = mapMessage.tracks ?? [];

    this.tagControl = new (getLeafletTagControlCtor())(this);
    this.tagControl.addTo(this.map);

    this.map.on('popupopen', (e) => {
      const popupContainer = e.popup.getElement();
      if (!popupContainer) return;
      const downloadLink = popupContainer.querySelector<HTMLAnchorElement>('.download-track-link');
      if (downloadLink) {
        let isDownloading = false;
        L.DomEvent.on(downloadLink, 'click', (e: Event) => {
          // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- Leaflet DomEvent passes a mouse event here
          const clickEvent = e as unknown as MouseEvent;
          if (isDownloading) {
            clickEvent.preventDefault();
            clickEvent.stopPropagation();
            return;
          }
          isDownloading = true;
          setTimeout(() => { isDownloading = false; }, 1000);
        });
      }
    });

    if (isIntersected) {
      this.loadTracks();
    }

    const bounds = this.boundsForMap(mapMessage);
    if (bounds.isValid() && !view) {
      this.map.fitBounds(bounds.pad(0.15), { animate: false });
    }
  }

  loadTracks(): void {
    if (this.tracksLoaded || !this.client || !this.popupRenderer) return;
    this.tracksLoaded = true;
    for (const track of this.tracksToLoad) {
      void this.renderTrack(track, this.client, this.page, this.mapName, this.popupRenderer);
    }
  }

  destroy(): void {
    this.map?.remove();
    this.map = null;
    this.tagControl = null;
    this.overlays = [];
    this.checkedTags.clear();
    this.allKnownTags.clear();
    this.failedTracks.clear();
    this.tagsInitialized = false;
    this.mapMessage = null;
    this.client = null;
    this.popupRenderer = null;
    this.tracksToLoad = [];
    this.tracksLoaded = false;
  }

  private renderMarker(marker: MapMarker, popupRenderer: PopupRenderer): void {
    if (!this.map || !marker.position) return;
    const leafletMarker = L.marker([marker.position.lat, marker.position.lon], {
      title: marker.label,
      icon: markerIcon(marker.color),
    }).addTo(this.map);
    this.bindPopup(leafletMarker, marker.popupMarkdown, popupRenderer);
    this.overlays.push({ layer: leafletMarker, tags: marker.tags ?? [] });
  }

  private renderPolygon(polygon: MapPolygon, popupRenderer: PopupRenderer): void {
    if (!this.map || polygon.points.length < 3) return;
    const layer = L.polygon(
      polygon.points.map(point => [point.lat, point.lon] as L.LatLngExpression),
      {
        color: polygon.strokeColor || '#2563eb',
        fillColor: polygon.fillColor || polygon.strokeColor || '#60a5fa',
        fillOpacity: 0.24,
      },
    ).addTo(this.map);
    this.bindPopup(layer, polygon.popupMarkdown, popupRenderer);
    this.overlays.push({ layer, tags: polygon.tags ?? [] });
  }

  private renderCircle(circle: MapCircle, popupRenderer: PopupRenderer): void {
    if (!this.map || !circle.center || circle.radiusMeters <= 0) return;
    const layer = L.circle([circle.center.lat, circle.center.lon], {
      radius: circle.radiusMeters,
      color: circle.strokeColor || '#047857',
      fillColor: circle.fillColor || circle.strokeColor || '#34d399',
      fillOpacity: 0.2,
    }).addTo(this.map);
    this.bindPopup(layer, circle.popupMarkdown, popupRenderer);
    this.overlays.push({ layer, tags: circle.tags ?? [] });
  }

  private async renderTrack(
    track: MapTrack,
    client: Client<typeof MapService>,
    page: string,
    mapName: string,
    // eslint-disable-next-line @typescript-eslint/no-unused-vars -- interface requires the parameter
    _popupRenderer: PopupRenderer
  ): Promise<void> {
    try {
      const response = await client.getTrackGeometry(create(GetTrackGeometryRequestSchema, {
        page,
        mapName,
        uid: track.metadata!.uid,
      }));
      if (!this.map) return;

      const segments = response.segments;
      if (!segments || segments.length === 0) return;

      const latLngsList: L.LatLngExpression[][] = segments.map(seg =>
        seg.points.map(pt => [pt.lat, pt.lon] as L.LatLngExpression)
      );

      const polyline = L.polyline(latLngsList, {
        color: track.color || '#3b82f6',
        weight: 4,
        opacity: 0.8,
      }).addTo(this.map);

      const totalMeters = calculateTrackDistanceMeters(latLngsList);
      let distanceHtml = '';
      if (totalMeters > 0) {
        const km = (totalMeters / 1000).toFixed(2);
        const miles = (totalMeters * 0.000621371).toFixed(2);
        distanceHtml = `<div style="margin-top: 4px; font-size: 0.85rem; color: #57606a;">Distance: ${km} km (${miles} miles)</div>`;
      }

      const downloadUrl = `/uploads/${encodeURIComponent(track.fileHash)}?filename=${encodeURIComponent(track.filename)}`;
      const popupHtml = `
        <div>
          <strong>${escapeHtml(track.label)}</strong>
          ${distanceHtml}
          <div style="margin-top: 5px;">
            <a class="download-track-link" href="${downloadUrl}" download="${escapeHtml(track.filename)}">Download Track</a>
          </div>
        </div>
      `;
      polyline.bindPopup(popupHtml);

      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- Leaflet returns Element but it is always an SVG/HTMLElement here
      const pathElement = polyline.getElement() as unknown as HTMLElement | null;
      if (pathElement) {
        pathElement.setAttribute('tabindex', '0');
        pathElement.setAttribute('role', 'button');
        pathElement.setAttribute('aria-label', `GPS Track: ${track.label}`);
        L.DomEvent.on(pathElement, 'keydown', (e: Event) => {
          // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- Leaflet DomEvent passes a keyboard event here
          const keyboardEvent = e as unknown as KeyboardEvent;
          if (keyboardEvent.key === 'Enter') {
            polyline.openPopup();
          }
        });
      }

      this.overlays.push({ layer: polyline, tags: track.tags ?? [] });
      
      this.tagControl?.updateTags();

      let shouldShow = false;
      if (track.tags && track.tags.length > 0) {
        shouldShow = track.tags.some(tag => this.checkedTags.has(tag));
      } else {
        shouldShow = this.checkedTags.has('untagged');
      }
      if (!shouldShow) {
        polyline.remove();
      }

      this.fitMapBounds();

    } catch {
      // Error surfaced via failedTracks set → tag control UI (lines 151-180).
      this.failedTracks.add(track.label);
      this.tagControl?.updateTags();
    }
  }

  public fitMapBounds(): void {
    if (!this.map || !this.mapMessage || this.mapMessage.view) return;
    const bounds = L.latLngBounds([]);
    for (const marker of this.mapMessage.markers ?? []) {
      if (marker.position) bounds.extend([marker.position.lat, marker.position.lon]);
    }
    for (const polygon of this.mapMessage.polygons ?? []) {
      for (const point of polygon.points) bounds.extend([point.lat, point.lon]);
    }
    for (const circle of this.mapMessage.circles ?? []) {
      if (circle.center) bounds.extend([circle.center.lat, circle.center.lon]);
    }
    for (const overlay of this.overlays) {
      if (overlay.layer instanceof L.Polyline) {
        bounds.extend(overlay.layer.getBounds());
      }
    }
    if (bounds.isValid()) {
      this.map.fitBounds(bounds.pad(0.15), { animate: false });
    }
  }

  public filterLayers(): void {
    if (!this.map) return;
    for (const overlay of this.overlays) {
      let shouldShow = false;
      if (overlay.tags && overlay.tags.length > 0) {
        shouldShow = overlay.tags.some(tag => this.checkedTags.has(tag));
      } else {
        shouldShow = this.checkedTags.has('untagged');
      }

      if (shouldShow) {
        if (!this.map.hasLayer(overlay.layer)) {
          overlay.layer.addTo(this.map);
        }
      } else {
        if (this.map.hasLayer(overlay.layer)) {
          overlay.layer.remove();
        }
      }
    }
  }

  private bindPopup(layer: L.Layer, popupMarkdown: string, popupRenderer: PopupRenderer): void {
    if (!popupMarkdown.trim()) return;
    void popupRenderer.render(popupMarkdown).then(renderedHtml => {
      layer.bindPopup(renderedHtml);
    });
  }

  private boundsForMap(mapMessage: WikiMapMessage): L.LatLngBounds {
    const bounds = L.latLngBounds([]);
    for (const marker of mapMessage.markers) {
      if (marker.position) bounds.extend([marker.position.lat, marker.position.lon]);
    }
    for (const polygon of mapMessage.polygons) {
      for (const point of polygon.points) bounds.extend([point.lat, point.lon]);
    }
    for (const circle of mapMessage.circles) {
      if (circle.center) bounds.extend([circle.center.lat, circle.center.lon]);
    }
    return bounds;
  }
}

function selectedTileLayer(tileLayers: TileLayer[], selectedId: number | undefined): TileLayer | undefined {
  return tileLayers.find(layer => layer.id === selectedId) ?? tileLayers[0];
}

function markerIcon(color: string | undefined | null): L.DivIcon {
  const fill = (color || '').trim() || '#dc2626';
  return L.divIcon({
    className: 'wiki-map-marker',
    html: `<span style="--wiki-map-marker-color:${escapeCssColor(fill)}"></span>`,
    iconSize: [24, 32],
    iconAnchor: [12, 32],
    popupAnchor: [0, -30],
  });
}

function escapeCssColor(value: string): string {
  return value.replace(/[^#(),.%\w\s-]/g, '');
}

/**
 * WikiMap renders a first-class wiki map by reading MapService.GetMap.
 *
 * @example
 * <wiki-map name="yard" page="garden_plan"></wiki-map>
 */
export class WikiMap extends LitElement {
  static override readonly styles = [
    foundationCSS,
    css`
      :host {
        display: block;
        isolation: isolate;
        margin: 1rem 0;
        position: relative;
        z-index: 0;
      }

      .map-shell {
        border: 1px solid var(--border-color, #d0d7de);
        min-height: 340px;
        position: relative;
      }

      .map-shell.drag-over {
        border: 2px dashed #0969da;
        background-color: rgba(9, 105, 218, 0.05);
      }

      .map-toolbar {
        align-items: center;
        background: rgb(255 255 255 / 0.92);
        border: 1px solid rgb(0 0 0 / 0.18);
        border-radius: 4px;
        box-shadow: 0 1px 4px rgb(0 0 0 / 0.2);
        display: flex;
        gap: 0.35rem;
        padding: 0.3rem;
        position: absolute;
        right: 10px;
        top: 10px;
        z-index: 1100;
      }

      .map-toolbar select {
        background: #fff;
        border: 1px solid #b6bec8;
        border-radius: 3px;
        color: #24292f;
        font: 13px/1.4 system-ui, sans-serif;
        max-width: min(16rem, 42vw);
        min-height: 1.9rem;
      }

      .map-canvas {
        aspect-ratio: var(--wiki-map-aspect-ratio, 16 / 9);
        height: auto;
        min-height: 340px;
        width: 100%;
      }

      .scroll-hint {
        background: rgb(36 41 47 / 0.88);
        border-radius: 4px;
        color: #fff;
        font: 14px/1.4 system-ui, sans-serif;
        left: 50%;
        max-width: min(24rem, calc(100% - 2rem));
        opacity: 0;
        padding: 0.65rem 0.85rem;
        pointer-events: none;
        position: absolute;
        text-align: center;
        top: 50%;
        transform: translate(-50%, -50%);
        transition: opacity 160ms ease;
        z-index: 1200;
      }

      .scroll-hint[visible] {
        opacity: 1;
      }

      .leaflet-container {
        background: #ddd;
        font: 12px/1.5 "Helvetica Neue", Arial, Helvetica, sans-serif;
        height: 100%;
        overflow: hidden;
        position: relative;
        touch-action: none;
        width: 100%;
      }

      .leaflet-pane,
      .leaflet-tile,
      .leaflet-marker-icon,
      .leaflet-marker-shadow,
      .leaflet-tile-container,
      .leaflet-pane > svg,
      .leaflet-pane > canvas,
      .leaflet-zoom-box,
      .leaflet-image-layer,
      .leaflet-layer {
        left: 0;
        position: absolute;
        top: 0;
      }

      .leaflet-container img.leaflet-tile,
      .leaflet-container .leaflet-marker-icon,
      .leaflet-container .leaflet-marker-shadow {
        max-height: none;
        max-width: none;
        user-select: none;
      }

      .leaflet-tile {
        border: 0;
        filter: inherit;
        height: 256px;
        image-rendering: auto;
        width: 256px;
      }

      .leaflet-tile-container {
        pointer-events: none;
      }

      .leaflet-map-pane {
        z-index: 400;
      }

      .leaflet-tile-pane {
        z-index: 200;
      }

      .leaflet-overlay-pane {
        z-index: 400;
      }

      .leaflet-shadow-pane {
        z-index: 500;
      }

      .leaflet-marker-pane {
        z-index: 600;
      }

      .leaflet-tooltip-pane {
        z-index: 650;
      }

      .leaflet-popup-pane {
        z-index: 700;
      }

      .leaflet-control {
        pointer-events: auto;
        position: relative;
        z-index: 800;
      }

      .leaflet-top,
      .leaflet-bottom {
        pointer-events: none;
        position: absolute;
        z-index: 1000;
      }

      .leaflet-top {
        top: 0;
      }

      .leaflet-right {
        right: 0;
      }

      .leaflet-bottom {
        bottom: 0;
      }

      .leaflet-left {
        left: 0;
      }

      .leaflet-control-zoom {
        border: 2px solid rgb(0 0 0 / 0.2);
        border-radius: 4px;
        margin-left: 10px;
        margin-top: 10px;
      }

      .leaflet-control-zoom a {
        background: #fff;
        border-bottom: 1px solid #ccc;
        color: #000;
        display: block;
        font: bold 18px/26px Arial, Helvetica, sans-serif;
        height: 26px;
        text-align: center;
        text-decoration: none;
        width: 26px;
      }

      .leaflet-control-zoom a:last-child {
        border-bottom: 0;
      }

      .leaflet-control-attribution {
        background: rgb(255 255 255 / 0.8);
        font-size: 11px;
        line-height: 1.4;
        margin: 0;
        padding: 0 5px;
      }

      .leaflet-bottom .leaflet-control {
        margin-bottom: 10px;
      }

      .leaflet-right .leaflet-control {
        margin-right: 10px;
      }

      .leaflet-popup {
        margin-bottom: 20px;
        position: absolute;
        text-align: center;
      }

      .leaflet-popup-content-wrapper {
        background: #fff;
        border-radius: 4px;
        box-shadow: 0 3px 14px rgb(0 0 0 / 0.4);
        padding: 1px;
        text-align: left;
      }

      .leaflet-popup-content {
        line-height: 1.4;
        margin: 13px 19px;
      }

      .leaflet-popup-tip-container {
        height: 20px;
        left: 50%;
        margin-left: -20px;
        overflow: hidden;
        position: absolute;
        width: 40px;
      }

      .leaflet-popup-tip {
        background: #fff;
        box-shadow: 0 3px 14px rgb(0 0 0 / 0.4);
        height: 17px;
        margin: -10px auto 0;
        padding: 1px;
        transform: rotate(45deg);
        width: 17px;
      }

      .state {
        align-items: center;
        color: var(--text-muted, #57606a);
        display: flex;
        min-height: 8rem;
        padding: 1rem;
      }

      .empty {
        color: var(--text-muted, #57606a);
        font-size: 0.95rem;
        padding: 1rem;
      }

      .wiki-map-marker span {
        background: var(--wiki-map-marker-color);
        border: 2px solid #fff;
        border-radius: 50% 50% 50% 0;
        box-shadow: 0 1px 4px rgb(0 0 0 / 0.35);
        display: block;
        height: 20px;
        transform: rotate(-45deg);
        width: 20px;
      }
    `,
  ];

  @property({ type: String })
  declare name: string;

  @property({ type: String })
  declare page: string;

  @property({ type: Boolean, reflect: true, attribute: 'tools-open' })
  declare toolsOpen: boolean;

  @state()
  declare loading: boolean;

  @state()
  private declare uploading: boolean;

  @state()
  declare error: AugmentedError | null;

  @state()
  private declare mapData: WikiMapMessage | null;

  @state()
  private declare showScrollHint: boolean;

  @state()
  private declare selectedFile: File | null;

  @state()
  private declare uploadError: string | null;

  @state()
  private declare showUploadPopover: boolean;

  @state()
  private declare isFileInvalid: boolean;

  @state()
  private isIntersected = false;

  private observer: IntersectionObserver | null = null;

  readonly client: Client<typeof MapService> = createClient(MapService, getGrpcWebTransport());
  readonly fileClient = createClient(FileStorageService, getGrpcWebTransport());
  
  private readonly popupMarkdownRenderer = new ChatMarkdownRenderer();
  markdownRenderer: PopupRenderer = {
    render: (markdown: string) => this.popupMarkdownRenderer.renderMarkdown(markdown, this.page),
  };
  rendererFactory: WikiMapRendererFactory = () => new LeafletWikiMapRenderer();
  private renderer: WikiMapRenderer | null = null;
  private scrollHintTimeoutId: number | undefined;

  constructor() {
    super();
    this.name = '';
    this.page = '';
    this.toolsOpen = false;
    this.loading = false;
    this.uploading = false;
    this.error = null;
    this.mapData = null;
    this.showScrollHint = false;
    this.selectedFile = null;
    this.uploadError = null;
    this.showUploadPopover = false;
    this.isFileInvalid = false;
    this.isIntersected = false;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    this.setupIntersectionObserver();
  }

  private setupIntersectionObserver(): void {
    if ('IntersectionObserver' in window) {
      this.observer = new IntersectionObserver(
        (entries) => {
          if (entries.some((entry) => entry.isIntersecting)) {
            this.isIntersected = true;
            this.disconnectIntersectionObserver();
            if (this.renderer) {
              this.renderer.loadTracks();
            }
          }
        },
        { rootMargin: '100px' }
      );
      this.observer.observe(this);
    } else {
      this.isIntersected = true;
    }
  }

  private disconnectIntersectionObserver(): void {
    if (this.observer) {
      this.observer.disconnect();
      this.observer = null;
    }
  }

  override disconnectedCallback(): void {
    if (this.scrollHintTimeoutId !== undefined) {
      window.clearTimeout(this.scrollHintTimeoutId);
      this.scrollHintTimeoutId = undefined;
    }
    this.disconnectIntersectionObserver();
    this.renderer?.destroy();
    this.renderer = null;
    super.disconnectedCallback();
  }

  override updated(changedProperties: Map<string, unknown>): void {
    if (changedProperties.has('name') || changedProperties.has('page')) {
      queueMicrotask(() => {
        void this.loadMap();
      });
    }
    if (changedProperties.has('mapData')) {
      this.renderLeafletMap();
    }
  }

  override render() {
    if (this.loading) {
      return html`<div class="state" role="status">Loading map</div>`;
    }
    if (this.error) {
      return html`<error-display .augmentedError=${this.error}></error-display>`;
    }
    if (!this.mapData) {
      return html`<div class="empty">Map unavailable</div>`;
    }
    return html`
      <div
        class="map-shell"
        @dragover=${this.handleDragOver}
        @dragleave=${this.handleDragLeave}
        @drop=${this.handleDrop}
      >
        ${this.renderTilesetSelector()}
        <div
          id="map-canvas"
          class="map-canvas"
          aria-label=${this.name}
          style=${styleMap({ '--wiki-map-aspect-ratio': aspectRatioCssValue(this.mapData.style?.aspectRatio) })}
          @wheel=${this.handleMapWheel}
        ></div>
        <div class="scroll-hint" ?visible=${this.showScrollHint}>Use Ctrl + scroll to zoom the map</div>

        <!-- Hidden file input for track upload -->
        <input
          type="file"
          id="gps-track-file-input"
          style="display: none;"
          accept=".gpx,.geojson"
          @change=${this.handleFileChange}
        />

        <!-- Tools Panel and Upload Popover -->
        ${this.renderToolsPanel()}
      </div>
    `;
  }

  private async loadMap(): Promise<void> {
    if (!this.isConnected) return;
    this.renderer?.destroy();
    this.renderer = null;
    this.mapData = null;
    if (!this.page || !this.name) {
      return;
    }
    const expectedPage = this.page;
    const expectedName = this.name;
    this.loading = true;
    this.error = null;
    try {
      const response = await this.client.getMap(create(GetMapRequestSchema, {
        page: expectedPage,
        mapName: expectedName,
        includeMarkers: true,
        includePolygons: true,
        includeCircles: true,
        includeTracks: true,
      }));
      if (this.page !== expectedPage || this.name !== expectedName) return;
      this.mapData = response.map ?? null;
    } catch (err: unknown) {
      if (this.page !== expectedPage || this.name !== expectedName) return;
      this.mapData = null;
      this.error = AugmentErrorService.augmentError(err, 'load map');
    } finally {
      if (this.page === expectedPage && this.name === expectedName) {
        this.loading = false;
      }
    }
  }

  private renderLeafletMap(): void {
    const container = this.renderRoot.querySelector<HTMLElement>('#map-canvas');
    if (!container || !this.mapData) return;
    this.renderer ??= this.rendererFactory();
    this.renderer.render(
      container,
      this.mapData,
      this.markdownRenderer,
      this.client,
      this.page,
      this.name,
      this.isIntersected
    );

    const leafletContainer = this.renderRoot.querySelector<HTMLElement>('#map-canvas');
    if (leafletContainer) {
      leafletContainer.removeEventListener('click', this.handleMapClick);
      leafletContainer.addEventListener('click', this.handleMapClick);
    }
  }

  private handleMapClick = (_event: Event): void => {
    this.toolsOpen = true;
  };

  private renderToolsPanel() {
    if (!this.toolsOpen) return null;
    return html`
      <div class="tools-panel" style="position: absolute; bottom: 10px; left: 50%; transform: translateX(-50%); background: rgba(255, 255, 255, 0.95); border: 1px solid #d0d7de; border-radius: 6px; box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15); padding: 6px 12px; display: flex; gap: 8px; z-index: 1100;">
        <button
          class="btn btn-sm"
          aria-label="Add GPS track"
          @click=${this.handleAddTrackClick}
        >
          Add GPS track
        </button>
      </div>
      ${this.renderUploadPopover()}
    `;
  }

  private renderUploadPopover() {
    if (!this.showUploadPopover) return null;
    return html`
      <div class="upload-popover" style="position: absolute; bottom: 50px; left: 50%; transform: translateX(-50%); background: white; border: 1px solid #d0d7de; padding: 1rem; border-radius: 4px; box-shadow: 0 2px 8px rgba(0,0,0,0.15); z-index: 1200; min-width: 250px;">
        <h4 style="margin: 0 0 0.5rem 0; font-size: 1rem;">Upload GPS Track</h4>
        
        ${this.uploadError
          ? html`<div class="error-message" style="color: var(--color-danger, #cf222e); font-size: 0.85rem; margin-bottom: 0.5rem;">${this.uploadError}</div>`
          : null}

        <form @submit=${this.handleUploadSubmit}>
          <div style="margin-bottom: 0.5rem;">
            <label style="display: block; font-size: 0.85rem; margin-bottom: 2px;">Label</label>
            <input
              type="text"
              name="label"
              class="form-control input-sm"
              style="width: 100%;"
              required
              .value=${this.selectedFile ? this.selectedFile.name.replace(/\.[^/.]+$/, "") : ''}
              ?disabled=${this.uploading}
            />
          </div>
          <div style="margin-bottom: 0.75rem;">
            <label style="display: block; font-size: 0.85rem; margin-bottom: 2px;">Tags (comma separated)</label>
            <input
              type="text"
              name="tags"
              class="form-control input-sm"
              style="width: 100%;"
              ?disabled=${this.uploading}
            />
          </div>
          <div style="display: flex; gap: 0.5rem; justify-content: flex-end;">
            <button
              type="button"
              class="btn btn-sm"
              @click=${this.cancelUpload}
              ?disabled=${this.uploading}
            >
              Cancel
            </button>
            <button
              type="submit"
              class="btn btn-sm btn-primary"
              ?disabled=${this.uploading || this.isFileInvalid}
            >
              ${this.uploading ? 'Uploading...' : 'Upload'}
            </button>
          </div>
        </form>
      </div>
    `;
  }

  private handleAddTrackClick(): void {
    const fileInput = this.renderRoot.querySelector<HTMLInputElement>('#gps-track-file-input');
    if (fileInput) {
      fileInput.click();
    }
  }

  private handleDragOver(event: DragEvent): void {
    event.preventDefault();
    event.stopPropagation();
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = 'copy';
    }
    const shell = this.renderRoot.querySelector('.map-shell');
    if (shell) {
      shell.classList.add('drag-over');
    }
  }

  private handleDragLeave(event: DragEvent): void {
    event.preventDefault();
    event.stopPropagation();
    const shell = this.renderRoot.querySelector('.map-shell');
    if (shell) {
      shell.classList.remove('drag-over');
    }
  }

  private handleDrop(event: DragEvent): void {
    event.preventDefault();
    event.stopPropagation();
    const shell = this.renderRoot.querySelector('.map-shell');
    if (shell) {
      shell.classList.remove('drag-over');
    }

    const file = event.dataTransfer?.files?.[0];
    if (!file) return;

    this.selectedFile = file;
    this.uploadError = null;
    this.isFileInvalid = false;
    this.showUploadPopover = true;

    const ext = file.name.split('.').pop()?.toLowerCase();
    if (!ext || !['gpx', 'kml', 'geojson'].includes(ext)) {
      this.uploadError = 'Unsupported file type. Please select a .gpx, .kml, or .geojson file.';
      this.isFileInvalid = true;
      return;
    }

    const MAX_SIZE = 10 * 1024 * 1024; // 10MB
    if (file.size > MAX_SIZE) {
      this.uploadError = 'File exceeds the maximum size of 10MB.';
      this.isFileInvalid = true;
      return;
    }
  }

  private handleFileChange(_event: Event): void {
    const input = this.renderRoot.querySelector<HTMLInputElement>('#gps-track-file-input');
    if (!input) return;
    const file = input.files?.[0];
    if (!file) return;

    this.selectedFile = file;
    this.uploadError = null;
    this.isFileInvalid = false;
    this.showUploadPopover = true;

    const ext = file.name.split('.').pop()?.toLowerCase();
    if (!ext || !['gpx', 'geojson'].includes(ext)) {
      this.uploadError = 'Unsupported file type. Please select a .gpx or .geojson file.';
      this.isFileInvalid = true;
      return;
    }

    const MAX_SIZE = 10 * 1024 * 1024; // 10MB
    if (file.size > MAX_SIZE) {
      this.uploadError = 'File exceeds the maximum size of 10MB.';
      this.isFileInvalid = true;
      return;
    }
  }

  private cancelUpload(): void {
    this.showUploadPopover = false;
    this.selectedFile = null;
    this.uploadError = null;
    this.isFileInvalid = false;
    const fileInput = this.renderRoot.querySelector<HTMLInputElement>('#gps-track-file-input');
    if (fileInput) {
      fileInput.value = '';
    }
  }

  private async handleUploadSubmit(event: Event): Promise<void> {
    event.preventDefault();
    if (this.isFileInvalid || !this.selectedFile) return;

    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- event target is the upload form
    const form = event.currentTarget as unknown as HTMLFormElement;
    const formData = new FormData(form);
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- form fields are text inputs
    const label = (formData.get('label') as unknown as string || '').trim();
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- form fields are text inputs
    const tagsString = (formData.get('tags') as unknown as string || '').trim();
    const tags = tagsString ? tagsString.split(',').map(t => t.trim()).filter(Boolean) : [];

    const file = this.selectedFile;
    this.uploading = true;
    this.uploadError = null;

    try {
      const content = new Uint8Array(await file.arrayBuffer());
      const uploadResp = await this.fileClient.uploadFile(create(UploadFileRequestSchema, {
        content,
        filename: file.name,
      }));

      const ext = file.name.split('.').pop()?.toLowerCase();
      const format = ext === 'geojson' ? 'GeoJSON' : 'GPX';

      const addTrackResp = await this.client.addTrack(create(AddTrackRequestSchema, {
        page: this.page,
        mapName: this.name,
        track: {
          label,
          fileHash: uploadResp.hash,
          format,
          tags,
          filename: file.name,
          color: '#10b981',
        },
      }));

      showToast(`${label} uploaded successfully`, 'success', 5);

      this.mapData = addTrackResp.map ?? null;
      this.showUploadPopover = false;
      this.selectedFile = null;
      this.requestUpdate();

    } catch (err: unknown) {
      this.uploadError = (err instanceof Error) ? err.message : String(err);
      this.selectedFile = file;
    } finally {
      this.uploading = false;
    }
  }

  private renderTilesetSelector() {
    const tileLayers = this.mapData?.style?.availableTileLayers ?? [];
    if (tileLayers.length <= 1) return null;
    const selectedId = selectedTileLayer(tileLayers, this.mapData?.style?.tileLayerId)?.id;
    return html`
      <div class="map-toolbar">
        <select aria-label="Tileset" @change=${this.handleTilesetChange}>
          ${tileLayers.map(layer => html`
            <option value=${String(layer.id)} ?selected=${layer.id === selectedId}>${layer.label}</option>
          `)}
        </select>
      </div>
    `;
  }

  private handleTilesetChange(event: Event): void {
    if (!(event.currentTarget instanceof HTMLSelectElement)) return;
    const select = event.currentTarget;
    const nextTileLayerId = Number(select.value);
    if (!this.mapData?.style || Number.isNaN(nextTileLayerId)) return;
    this.mapData.style.tileLayerId = nextTileLayerId;
    this.renderLeafletMap();
    this.requestUpdate();
  }

  private handleMapWheel(event: WheelEvent): void {
    if (event.ctrlKey || event.metaKey) return;
    event.stopImmediatePropagation();
    this.showScrollHint = true;
    if (this.scrollHintTimeoutId !== undefined) {
      window.clearTimeout(this.scrollHintTimeoutId);
    }
    this.scrollHintTimeoutId = window.setTimeout(() => {
      this.showScrollHint = false;
      this.scrollHintTimeoutId = undefined;
    }, 1600);
  }
}

customElements.define('wiki-map', WikiMap);

function aspectRatioCssValue(value: string | undefined): string {
  if (!value) return '16 / 9';
  const match = /^([1-9][0-9]{0,2}):([1-9][0-9]{0,2})$/.exec(value);
  if (!match) return '16 / 9';
  return `${match[1]} / ${match[2]}`;
}

function calculateTrackDistanceMeters(latLngsList: L.LatLngExpression[][]): number {
  let totalMeters = 0;
  for (const segment of latLngsList) {
    for (let i = 0; i < segment.length - 1; i++) {
      const rawP1 = segment[i];
      const rawP2 = segment[i + 1];
      if (rawP1 == null || rawP2 == null) continue;
      const p1 = L.latLng(rawP1);
      const p2 = L.latLng(rawP2);
      totalMeters += p1.distanceTo(p2);
    }
  }
  return totalMeters;
}

declare global {
  interface HTMLElementTagNameMap {
    'wiki-map': WikiMap;
  }
}
