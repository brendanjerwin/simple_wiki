import { expect, waitUntil } from '@open-wc/testing';
import { create } from '@bufbuild/protobuf';
import sinon, { type SinonStub } from 'sinon';
import './wiki-map.js';
import type { PopupRenderer, WikiMap, WikiMapRenderer } from './wiki-map.js';
import {
  GeoPointSchema,
  GetMapResponseSchema,
  MapMarkerSchema,
  MapSchema,
  MapStyleSchema,
  TileLayerId,
  TileLayerSchema,
  type GetMapRequest,
  type Map as WikiMapMessage,
} from '../gen/api/v1/map_pb.js';

class StubRenderer implements WikiMapRenderer {
  renderStub = sinon.stub();
  destroyStub = sinon.stub();

  render(container: HTMLElement, map: WikiMapMessage, popupRenderer: PopupRenderer): void {
    this.renderStub(container, map, popupRenderer);
  }

  destroy(): void {
    this.destroyStub();
  }
}

class StylingProbeRenderer implements WikiMapRenderer {
  render(container: HTMLElement): void {
    container.innerHTML = `
      <div class="leaflet-pane"></div>
      <div class="leaflet-marker-icon wiki-map-marker"><span></span></div>
    `;
  }

  destroy(): void {}
}

interface StubMapClient {
  getMap: SinonStub;
}

function clientOf(el: WikiMap): StubMapClient {
  return (el as unknown as { client: StubMapClient }).client;
}

function setClient(el: WikiMap, client: StubMapClient): void {
  Object.defineProperty(el, 'client', { value: client });
}

function sampleMap(): WikiMapMessage {
  return create(MapSchema, {
    page: 'garden_plan',
    name: 'yard',
    style: create(MapStyleSchema, {
      tileLayerId: TileLayerId.OPENTOPOMAP,
      aspectRatio: '3:2',
      availableTileLayers: [
        create(TileLayerSchema, {
          id: TileLayerId.OPENSTREETMAP,
          label: 'OpenStreetMap',
          urlTemplate: 'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
          attributionHtml: 'OpenStreetMap contributors',
        }),
        create(TileLayerSchema, {
          id: TileLayerId.OPENTOPOMAP,
          label: 'OpenTopoMap',
          urlTemplate: 'https://tile.opentopomap.org/{z}/{x}/{y}.png',
          attributionHtml: 'OpenTopoMap contributors',
        }),
        create(TileLayerSchema, {
          id: TileLayerId.ESRI_WORLD_IMAGERY,
          label: 'Esri World Imagery',
          urlTemplate: 'https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}',
          attributionHtml: 'Esri contributors',
        }),
      ],
    }),
    markers: [
      create(MapMarkerSchema, {
        label: 'Shed',
        position: create(GeoPointSchema, { lat: 41.1, lon: -72.2 }),
        popupMarkdown: '[Shed](shed)',
      }),
    ],
  });
}

describe('WikiMap', () => {
  let el: WikiMap;
  let renderer: StubRenderer;
  let client: StubMapClient;
  let renderedMap: WikiMapMessage;

  beforeEach(() => {
    renderedMap = sampleMap();
    renderer = new StubRenderer();
    client = {
      getMap: sinon.stub().resolves(create(GetMapResponseSchema, { map: renderedMap })),
    };
  });

  describe('when connected with page and name', () => {
    beforeEach(async () => {
      el = document.createElement('wiki-map') as WikiMap;
      el.page = 'garden_plan';
      el.name = 'yard';
      setClient(el, client);
      el.rendererFactory = () => renderer;
      el.markdownRenderer = { render: sinon.stub().resolves('<a href="/shed">Shed</a>') };
      document.body.appendChild(el);
      await el.updateComplete;
      await waitUntil(() => renderer.renderStub.calledOnce);
    });

    it('should request the named map from MapService', () => {
      expect(clientOf(el).getMap.calledOnce).to.equal(true);
      const request = client.getMap.firstCall.args[0] as GetMapRequest;
      expect(request.page).to.equal('garden_plan');
      expect(request.mapName).to.equal('yard');
    });

    it('should request all element types', () => {
      const request = client.getMap.firstCall.args[0] as GetMapRequest;
      expect(request.includeMarkers).to.equal(true);
      expect(request.includePolygons).to.equal(true);
      expect(request.includeCircles).to.equal(true);
    });

    it('should render the returned map through the renderer', () => {
      expect(renderer.renderStub.firstCall.args[1]).to.equal(renderedMap);
    });

    it('should render a tileset selector', () => {
      const selector = el.shadowRoot?.querySelector<HTMLSelectElement>('select[aria-label="Tileset"]');
      expect(selector).not.to.equal(null);
    });

    it('should select the default tileset from map style', () => {
      const selector = el.shadowRoot?.querySelector<HTMLSelectElement>('select[aria-label="Tileset"]');
      expect(selector?.value).to.equal(String(TileLayerId.OPENTOPOMAP));
    });

    it('should list available tilesets', () => {
      const options = [...(el.shadowRoot?.querySelectorAll<HTMLOptionElement>('select[aria-label="Tileset"] option') ?? [])];
      expect(options.map(option => option.textContent)).to.deep.equal([
        'OpenStreetMap',
        'OpenTopoMap',
        'Esri World Imagery',
      ]);
    });

    it('should use the configured map aspect ratio', () => {
      const canvas = el.shadowRoot?.querySelector<HTMLElement>('#map-canvas');
      expect(canvas?.style.getPropertyValue('--wiki-map-aspect-ratio')).to.equal('3 / 2');
    });

    describe('when scrolling the map without a zoom modifier', () => {
      beforeEach(async () => {
        const canvas = el.shadowRoot?.querySelector<HTMLElement>('#map-canvas');
        canvas!.dispatchEvent(new WheelEvent('wheel', { bubbles: true, deltaY: 120 }));
        await el.updateComplete;
      });

      it('should show the control-scroll zoom hint', () => {
        const hint = el.shadowRoot?.querySelector<HTMLElement>('.scroll-hint');
        expect(hint?.hasAttribute('visible')).to.equal(true);
      });
    });

    describe('when control-scrolling the map', () => {
      beforeEach(async () => {
        const canvas = el.shadowRoot?.querySelector<HTMLElement>('#map-canvas');
        canvas!.dispatchEvent(new WheelEvent('wheel', { bubbles: true, deltaY: 120, ctrlKey: true }));
        await el.updateComplete;
      });

      it('should leave the zoom hint hidden', () => {
        const hint = el.shadowRoot?.querySelector<HTMLElement>('.scroll-hint');
        expect(hint?.hasAttribute('visible')).to.equal(false);
      });
    });

    describe('when choosing another tileset', () => {
      beforeEach(async () => {
        const selector = el.shadowRoot?.querySelector<HTMLSelectElement>('select[aria-label="Tileset"]');
        selector!.value = String(TileLayerId.ESRI_WORLD_IMAGERY);
        selector!.dispatchEvent(new Event('change', { bubbles: true }));
        await el.updateComplete;
      });

      it('should render the map again', () => {
        expect(renderer.renderStub.callCount).to.equal(2);
      });

      it('should use the selected tileset for the current view', () => {
        const mapMessage = renderer.renderStub.secondCall.args[1] as WikiMapMessage;
        expect(mapMessage.style?.tileLayerId).to.equal(TileLayerId.ESRI_WORLD_IMAGERY);
      });
    });

    it('should provide a popup markdown renderer', async () => {
      const popupRenderer = renderer.renderStub.firstCall.args[2] as PopupRenderer;
      const htmlResult = await popupRenderer.render('[Shed](shed)');
      expect(htmlResult).to.equal('<a href="/shed">Shed</a>');
    });

    afterEach(() => {
      el.remove();
    });
  });

  describe('when using the default Leaflet renderer', () => {
    beforeEach(async () => {
      el = document.createElement('wiki-map') as WikiMap;
      el.page = 'garden_plan';
      el.name = 'yard';
      setClient(el, client);
      el.markdownRenderer = { render: sinon.stub().resolves('<a href="/shed">Shed</a>') };
      el.rendererFactory = () => new StylingProbeRenderer();
      document.body.appendChild(el);
      await el.updateComplete;
      await waitUntil(() => el.shadowRoot?.querySelector('.wiki-map-marker') !== null);
    });

    it('should position Leaflet panes within the map canvas', () => {
      const pane = el.shadowRoot?.querySelector<HTMLElement>('.leaflet-pane');
      expect(getComputedStyle(pane!).position).to.equal('absolute');
    });

    it('should position marker icons within the map canvas', () => {
      const marker = el.shadowRoot?.querySelector<HTMLElement>('.wiki-map-marker');
      expect(getComputedStyle(marker!).position).to.equal('absolute');
    });

    it('should keep Leaflet layers inside the page content stacking level', () => {
      const styles = getComputedStyle(el);
      expect(styles.isolation).to.equal('isolate');
      expect(styles.position).to.equal('relative');
      expect(styles.zIndex).to.equal('0');
    });

    afterEach(() => {
      el.remove();
    });
  });

  describe('when MapService rejects the request', () => {
    beforeEach(async () => {
      client.getMap.rejects(new Error('map missing'));
      el = document.createElement('wiki-map') as WikiMap;
      el.page = 'garden_plan';
      el.name = 'yard';
      setClient(el, client);
      el.rendererFactory = () => renderer;
      document.body.appendChild(el);
      await waitUntil(() => client.getMap.calledOnce);
      await el.updateComplete;
    });

    it('should render an error state', () => {
      expect(el.shadowRoot?.querySelector('error-display')).not.to.equal(null);
    });

    it('should not render Leaflet', () => {
      expect(renderer.renderStub.called).to.equal(false);
    });

    afterEach(() => {
      el.remove();
    });
  });
});
