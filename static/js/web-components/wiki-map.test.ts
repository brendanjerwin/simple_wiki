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
      tileLayerId: TileLayerId.OPENSTREETMAP,
      availableTileLayers: [
        create(TileLayerSchema, {
          id: TileLayerId.OPENSTREETMAP,
          label: 'OpenStreetMap',
          urlTemplate: 'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
          attributionHtml: 'OpenStreetMap contributors',
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
      expect(clientOf(el).getMap).to.have.been.calledOnce;
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

    it('should provide a popup markdown renderer', async () => {
      const popupRenderer = renderer.renderStub.firstCall.args[2] as PopupRenderer;
      const htmlResult = await popupRenderer.render('[Shed](shed)');
      expect(htmlResult).to.equal('<a href="/shed">Shed</a>');
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
      expect(el.shadowRoot?.querySelector('error-display')).to.exist;
    });

    it('should not render Leaflet', () => {
      expect(renderer.renderStub).not.to.have.been.called;
    });

    afterEach(() => {
      el.remove();
    });
  });
});
