import { expect, waitUntil } from '@open-wc/testing';
import { create } from '@bufbuild/protobuf';
import sinon, { type SinonStub } from 'sinon';
import './wiki-map.js';
import { LeafletWikiMapRenderer, type PopupRenderer, type WikiMap, type WikiMapRenderer } from './wiki-map.js';
import { setLeafletOverride, resetLeafletOverrides } from './leaflet-accessor.js';
import {
  GeoPointSchema,
  GetMapResponseSchema,
  MapMarkerSchema,
  MapSchema,
  MapStyleSchema,
  MapTrackSchema,
  TileLayerId,
  TileLayerSchema,
  type GetMapRequest,
  type Map as WikiMapMessage,
} from '../gen/api/v1/map_pb.js';

class StubRenderer implements WikiMapRenderer {
  renderStub = sinon.stub();
  destroyStub = sinon.stub();
  loadTracksStub = sinon.stub();

  render(
    container: HTMLElement,
    map: WikiMapMessage,
    popupRenderer: PopupRenderer,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- test stub for MapService client
    client: any,
    page: string,
    mapName: string,
    isIntersected: boolean
  ): void {
    this.renderStub(container, map, popupRenderer, client, page, mapName, isIntersected);
  }

  loadTracks(): void {
    this.loadTracksStub();
  }

  destroy(): void {
    this.destroyStub();
  }
}

class StylingProbeRenderer implements WikiMapRenderer {
  render(
    container: HTMLElement,
    _map: WikiMapMessage,
    _popupRenderer: PopupRenderer,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- test stub for MapService client
    _client: any,
    _page: string,
    _mapName: string,
    _isIntersected: boolean
  ): void {
    container.innerHTML = `
      <div class="leaflet-pane"></div>
      <div class="leaflet-marker-icon wiki-map-marker"><span></span></div>
    `;
  }

  loadTracks(): void {}
  destroy(): void {}
}

interface StubMapClient {
  getMap: SinonStub;
}

interface Deferred<T> {
  promise: Promise<T>;
  resolve: (value: T) => void;
}

function deferred<T>(): Deferred<T> {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>(innerResolve => {
    resolve = innerResolve;
  });
  return { promise, resolve };
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

  describe('when reloading after a rendered map', () => {
    beforeEach(async () => {
      el = document.createElement('wiki-map') as WikiMap;
      el.page = 'garden_plan';
      el.name = 'yard';
      setClient(el, client);
      el.rendererFactory = () => renderer;
      document.body.appendChild(el);
      await waitUntil(() => renderer.renderStub.calledOnce);

      el.name = 'front_yard';
      await waitUntil(() => renderer.destroyStub.calledOnce);
    });

    it('should destroy the old renderer', () => {
      expect(renderer.destroyStub.calledOnce).to.equal(true);
    });

    afterEach(() => {
      el.remove();
    });
  });

  describe('when an older map request resolves after a newer request', () => {
    let firstLoad: Deferred<unknown>;
    let secondLoad: Deferred<unknown>;
    let newerMap: WikiMapMessage;

    beforeEach(async () => {
      firstLoad = deferred<unknown>();
      secondLoad = deferred<unknown>();
      newerMap = sampleMap();
      newerMap.name = 'front_yard';
      client.getMap.onFirstCall().returns(firstLoad.promise);
      client.getMap.onSecondCall().returns(secondLoad.promise);

      el = document.createElement('wiki-map') as WikiMap;
      el.page = 'garden_plan';
      el.name = 'yard';
      setClient(el, client);
      el.rendererFactory = () => renderer;
      document.body.appendChild(el);
      await waitUntil(() => client.getMap.calledOnce);

      el.name = 'front_yard';
      await waitUntil(() => client.getMap.calledTwice);
      secondLoad.resolve(create(GetMapResponseSchema, { map: newerMap }));
      await waitUntil(() => renderer.renderStub.calledOnce);
      firstLoad.resolve(create(GetMapResponseSchema, { map: renderedMap }));
      await Promise.resolve();
      await el.updateComplete;
    });

    it('should render only the newer response', () => {
      expect(renderer.renderStub.calledOnce).to.equal(true);
      expect((renderer.renderStub.firstCall.args[1] as WikiMapMessage).name).to.equal('front_yard');
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
      await waitUntil(() => el.shadowRoot?.querySelector('error-display') !== null);
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
  // stubLeaflet replaces Leaflet APIs with no-op stubs via the Proxy-based
  // accessor, allowing tests to exercise LeafletWikiMapRenderer without a
  // real Leaflet instance. The Proxy intercepts sinon.stub calls on the
  // mutable Leaflet object.
  function stubLeaflet(): void {
    const mockLayer = {
      addTo: sinon.stub().returnsThis(),
      remove: sinon.stub(),
      setStyle: sinon.stub(),
      getElement: sinon.stub().returns(null),
      on: sinon.stub(),
      off: sinon.stub(),
      bindPopup: sinon.stub().returnsThis(),
      openPopup: sinon.stub(),
    };
    const mockMap = {
      setView: sinon.stub(),
      on: sinon.stub(),
      off: sinon.stub(),
      remove: sinon.stub(),
      addLayer: sinon.stub(),
      removeLayer: sinon.stub(),
      getBounds: sinon.stub().returns({ extend: sinon.stub() }),
      fitBounds: sinon.stub(),
      hasLayer: sinon.stub().returns(false),
      panTo: sinon.stub(),
    };
    setLeafletOverride('map', sinon.stub().returns(mockMap));
    // L.Control is a class that LeafletTagControl extends; provide a minimal
    // base class so the extends works in the test environment.
    setLeafletOverride('Control', class {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any -- Leaflet Control constructor accepts options
      constructor(_options?: any) {}
      addTo(_map: unknown): this { return this; }
      remove(): void {}
    });
    setLeafletOverride('tileLayer', sinon.stub().returns(mockLayer));
    setLeafletOverride('marker', sinon.stub().returns(mockLayer));
    setLeafletOverride('polygon', sinon.stub().returns(mockLayer));
    setLeafletOverride('circle', sinon.stub().returns(mockLayer));
    setLeafletOverride('polyline', sinon.stub().returns(mockLayer));
    setLeafletOverride('divIcon', sinon.stub().returns({}));
    setLeafletOverride('latLng', sinon.stub().returns({ lat: 0, lng: 0, distanceTo: sinon.stub().returns(0) }));
    setLeafletOverride('latLngBounds', sinon.stub().returns({ extend: sinon.stub(), isValid: sinon.stub().returns(false) }));
    // DomUtil and DomEvent are objects on the namespace; stub their methods
    // via setLeafletOverride by replacing the whole object with a stub.
    setLeafletOverride('DomUtil', {
      create: sinon.stub().callsFake((tag: string, _className?: string, container?: HTMLElement) => {
        const el = document.createElement(tag);
        if (container) container.appendChild(el);
        return el;
      }),
      removeClass: sinon.stub(),
      addClass: sinon.stub(),
    });
    setLeafletOverride('DomEvent', {
      on: sinon.stub(),
      disableClickPropagation: sinon.stub(),
      disableScrollPropagation: sinon.stub(),
    });
  }



  describe('Milestone 5 Refinement behaviors', () => {
    describe('LeafletWikiMapRenderer lazy loading & tag control state preservation', () => {
      let mapContainer: HTMLElement;
      let leafletRenderer: LeafletWikiMapRenderer;
      // eslint-disable-next-line @typescript-eslint/no-explicit-any -- test stub for MapService client
      let mockClient: any;
      let mapMsg: WikiMapMessage;

      beforeEach(() => {
        stubLeaflet();
        // IntersectionObserver is not needed for direct renderer tests;
        // the renderer receives isIntersected as a parameter.

        mapContainer = document.createElement('div');
        document.body.appendChild(mapContainer);
        leafletRenderer = new LeafletWikiMapRenderer();
        mockClient = {
          getTrackGeometry: sinon.stub().resolves({ segments: [] }),
        };

        mapMsg = sampleMap();
        mapMsg.tracks = [
          create(MapTrackSchema, {
            // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- minimal MapTrackMetadata stub for test
            metadata: { uid: 'track-1' } as unknown as { uid: string },
            label: 'Scenic Route',
            fileHash: 'hash123',
            format: 'GPX',
            color: '#3b82f6',
            tags: ['hiking'],
            filename: 'route.gpx',
          }),
        ];
      });

      afterEach(() => {
        sinon.restore();
        resetLeafletOverrides();
        leafletRenderer.destroy();
        mapContainer.remove();
      });

      describe('lazy loading track geometries', () => {
        beforeEach(() => {
          leafletRenderer.render(
            mapContainer,
            mapMsg,
            { render: sinon.stub().resolves('') },
            mockClient,
            'garden_plan',
            'yard',
            false
          );
        });

        it('should not render track geometry eagerly', () => {
          expect(mockClient.getTrackGeometry.called).to.equal(false);
        });

        describe('when renderer is told to load tracks', () => {
          beforeEach(async () => {
            leafletRenderer.loadTracks();
            await waitUntil(() => mockClient.getTrackGeometry.calledOnce);
          });

          it('should render track geometry', () => {
            expect(mockClient.getTrackGeometry.calledWith(sinon.match({ uid: 'track-1' }))).to.equal(true);
          });
        });
      });

      describe('when destroying the renderer', () => {
        beforeEach(() => {
          leafletRenderer.render(
            mapContainer,
            mapMsg,
            { render: sinon.stub().resolves('') },
            mockClient,
            'garden_plan',
            'yard',
            false
          );
          leafletRenderer.destroy();
        });

        it('should remove the Leaflet map', () => {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any -- accessing private map for test verification
          expect((leafletRenderer as unknown as { map: unknown }).map).to.equal(null);
        });
      });

      describe('when updating tags', () => {
        beforeEach(() => {
          leafletRenderer.render(
            mapContainer,
            mapMsg,
            { render: sinon.stub().resolves('') },
            mockClient,
            'garden_plan',
            'yard',
            false
          );
          // Initially check tag
          leafletRenderer.checkedTags.add('scenic');
          leafletRenderer.tagControl?.updateTags();

          // Uncheck it manually
          leafletRenderer.checkedTags.delete('scenic');
          
          // updateTags should not re-add it
          leafletRenderer.tagControl?.updateTags();
        });

        it('should not reset checkedTags state', () => {
          expect(leafletRenderer.checkedTags.has('scenic')).to.equal(false);
        });
      });
    });

    describe('Upload error handling', () => {
      let uploadEl: WikiMap;
      // eslint-disable-next-line @typescript-eslint/no-explicit-any -- test stub for FileStorageService client
      let mockFileClient: any;
      // eslint-disable-next-line @typescript-eslint/no-explicit-any -- test stub for MapService client
      let mockMapClient: any;
      let testFile: File;

      beforeEach(async () => {
        stubLeaflet();
        mockFileClient = {
          uploadFile: sinon.stub().rejects(new Error('Network upload failure')),
        };
        mockMapClient = {
          getMap: sinon.stub().resolves(create(GetMapResponseSchema, { map: sampleMap() })),
        };

        uploadEl = document.createElement('wiki-map') as WikiMap;
        uploadEl.page = 'garden_plan';
        uploadEl.name = 'yard';
        setClient(uploadEl, mockMapClient);
        Object.defineProperty(uploadEl, 'fileClient', { value: mockFileClient });
        document.body.appendChild(uploadEl);
        await uploadEl.updateComplete;
        await waitUntil(() => uploadEl.shadowRoot?.querySelector('#gps-track-file-input') !== null);
        await uploadEl.updateComplete;

        testFile = new File(['dummy content'], 'route.gpx', { type: 'application/gpx+xml' });
      });

      afterEach(() => {
        sinon.restore();
        resetLeafletOverrides();
        uploadEl.remove();
      });

      describe('when a file is selected', () => {
        beforeEach(async () => {
          await waitUntil(() => uploadEl.shadowRoot?.querySelector('#gps-track-file-input') != null, 'file input should appear', { timeout: 5000 });
          // Setting FileList on an input is unreliable in the test runner
          // (files is a read-only getter on HTMLInputElement.prototype).
          // Instead, set the internal state directly to simulate a successful
          // file selection, then verify the popover renders.
          // eslint-disable-next-line @typescript-eslint/no-explicit-any -- accessing private state for test setup
          (uploadEl as unknown as { selectedFile: File | null; showUploadPopover: boolean }).selectedFile = testFile;
          // eslint-disable-next-line @typescript-eslint/no-explicit-any -- accessing private state for test setup
          (uploadEl as unknown as { showUploadPopover: boolean; toolsOpen: boolean }).showUploadPopover = true;
          (uploadEl as unknown as { toolsOpen: boolean }).toolsOpen = true;
          uploadEl.requestUpdate();
          await uploadEl.updateComplete;
        });

        it('should display the upload popover', () => {
          expect(uploadEl.shadowRoot?.querySelector('.upload-popover')).not.to.equal(null);
        });

        describe('when the upload form is submitted and fails', () => {
          beforeEach(async () => {
            const form = uploadEl.shadowRoot?.querySelector('.upload-popover form') as HTMLFormElement;
            form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
            await waitUntil(() => ((uploadEl as unknown as { uploadError: string | null }).uploadError) !== null);
          });

          it('should keep the upload popover visible', () => {
            expect(uploadEl.shadowRoot?.querySelector('.upload-popover')).not.to.equal(null);
          });

          it('should display the upload error message on the popover', () => {
            expect((uploadEl as unknown as { uploadError: string | null }).uploadError).to.equal('Network upload failure');
          });

          it('should not set a global map error or crash the map component', () => {
            expect(uploadEl.error).to.equal(null);
          });
        });
      });
    });
  });
});
