import { css, html, LitElement } from 'lit';
import { property, state } from 'lit/decorators.js';
import { createClient, type Client } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import * as L from 'leaflet';
import { getGrpcWebTransport } from './grpc-transport.js';
import {
  GetMapRequestSchema,
  MapService,
  type Map as WikiMapMessage,
  type MapCircle,
  type MapMarker,
  type MapPolygon,
  type TileLayer,
} from '../gen/api/v1/map_pb.js';
import { ChatMarkdownRenderer } from './chat-markdown-renderer.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
import { foundationCSS } from './shared-styles.js';
import './error-display.js';

export interface WikiMapRenderer {
  render(container: HTMLElement, map: WikiMapMessage, popupRenderer: PopupRenderer): void;
  destroy(): void;
}

export interface PopupRenderer {
  render(markdown: string): Promise<string>;
}

export type WikiMapRendererFactory = () => WikiMapRenderer;

export class LeafletWikiMapRenderer implements WikiMapRenderer {
  private map: L.Map | null = null;

  render(container: HTMLElement, mapMessage: WikiMapMessage, popupRenderer: PopupRenderer): void {
    this.destroy();
    this.map = L.map(container, {
      zoomControl: true,
      scrollWheelZoom: false,
      preferCanvas: true,
    });

    const view = mapMessage.view;
    const markerPoints = mapMessage.markers.map(marker => marker.position).filter(point => point !== undefined);
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

    for (const marker of mapMessage.markers) {
      this.renderMarker(marker, popupRenderer);
    }
    for (const polygon of mapMessage.polygons) {
      this.renderPolygon(polygon, popupRenderer);
    }
    for (const circle of mapMessage.circles) {
      this.renderCircle(circle, popupRenderer);
    }

    const bounds = this.boundsForMap(mapMessage);
    if (bounds.isValid() && !view) {
      this.map.fitBounds(bounds.pad(0.15), { animate: false });
    }
  }

  destroy(): void {
    this.map?.remove();
    this.map = null;
  }

  private renderMarker(marker: MapMarker, popupRenderer: PopupRenderer): void {
    if (!this.map || !marker.position) return;
    const leafletMarker = L.marker([marker.position.lat, marker.position.lon], {
      title: marker.label,
      icon: markerIcon(marker.color),
    }).addTo(this.map);
    this.bindPopup(leafletMarker, marker.popupMarkdown, popupRenderer);
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

function markerIcon(color: string): L.DivIcon {
  const fill = color.trim() || '#dc2626';
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

      .map-canvas {
        height: min(62vh, 520px);
        min-height: 340px;
        width: 100%;
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

  @state()
  declare loading: boolean;

  @state()
  declare error: AugmentedError | null;

  @state()
  private declare mapData: WikiMapMessage | null;

  readonly client: Client<typeof MapService> = createClient(MapService, getGrpcWebTransport());
  markdownRenderer: PopupRenderer = {
    render: (markdown: string) => new ChatMarkdownRenderer().renderMarkdown(markdown, this.page),
  };
  rendererFactory: WikiMapRendererFactory = () => new LeafletWikiMapRenderer();
  private renderer: WikiMapRenderer | null = null;

  constructor() {
    super();
    this.name = '';
    this.page = '';
    this.loading = false;
    this.error = null;
    this.mapData = null;
  }

  override disconnectedCallback(): void {
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
      <div class="map-shell">
        <div id="map-canvas" class="map-canvas" aria-label=${this.name}></div>
      </div>
    `;
  }

  private async loadMap(): Promise<void> {
    if (!this.isConnected) return;
    if (!this.page || !this.name) {
      this.mapData = null;
      return;
    }
    this.loading = true;
    this.error = null;
    try {
      const response = await this.client.getMap(create(GetMapRequestSchema, {
        page: this.page,
        mapName: this.name,
        includeMarkers: true,
        includePolygons: true,
        includeCircles: true,
      }));
      this.mapData = response.map ?? null;
    } catch (err: unknown) {
      this.mapData = null;
      this.error = AugmentErrorService.augmentError(err, 'load map');
    } finally {
      this.loading = false;
    }
  }

  private renderLeafletMap(): void {
    const container = this.renderRoot.querySelector<HTMLElement>('#map-canvas');
    if (!container || !this.mapData) return;
    this.renderer ??= this.rendererFactory();
    this.renderer.render(container, this.mapData, this.markdownRenderer);
  }
}

customElements.define('wiki-map', WikiMap);

declare global {
  interface HTMLElementTagNameMap {
    'wiki-map': WikiMap;
  }
}
