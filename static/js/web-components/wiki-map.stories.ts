import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { create } from '@bufbuild/protobuf';
import { html } from 'lit';
import { ref } from 'lit/directives/ref.js';
import { action } from 'storybook/actions';
import './wiki-map.js';
import type { WikiMap, WikiMapRenderer, PopupRenderer } from './wiki-map.js';
import {
  GeoPointSchema,
  GetMapResponseSchema,
  MapCircleSchema,
  MapMarkerSchema,
  MapPolygonSchema,
  MapSchema,
  MapStyleSchema,
  TileLayerId,
  TileLayerSchema,
  type Map as WikiMapMessage,
} from '../gen/api/v1/map_pb.js';

const meta: Meta = {
  title: 'Components/WikiMap',
  tags: ['autodocs'],
  component: 'wiki-map',
  parameters: {
    docs: {
      description: {
        component: 'Renders first-class wiki maps through MapService.GetMap.',
      },
    },
  },
};

export default meta;
type Story = StoryObj;

class StoryRenderer implements WikiMapRenderer {
  render(
    container: HTMLElement,
    map: WikiMapMessage,
    _popupRenderer: PopupRenderer,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- storybook stub for MapService client
    _client: any,
    _page: string,
    _mapName: string,
    _isIntersected: boolean
  ): void {
    container.innerHTML = `
      <div style="height:100%;display:grid;place-items:center;background:#eef6f1;color:#1f2937;">
        <div>
          <strong>${map.name}</strong><br>
          ${map.markers.length} markers, ${map.polygons.length} polygons, ${map.circles.length} circles
        </div>
      </div>
    `;
    action('map-rendered')({ name: map.name, markers: map.markers.length });
  }

  loadTracks(): void {}
  destroy(): void {}
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
      create(MapMarkerSchema, {
        label: 'Compost',
        position: create(GeoPointSchema, { lat: 41.101, lon: -72.203 }),
      }),
    ],
    polygons: [
      create(MapPolygonSchema, {
        label: 'Vegetable beds',
        points: [
          create(GeoPointSchema, { lat: 41.1, lon: -72.2 }),
          create(GeoPointSchema, { lat: 41.102, lon: -72.2 }),
          create(GeoPointSchema, { lat: 41.102, lon: -72.204 }),
        ],
        strokeColor: '#2563eb',
        fillColor: '#93c5fd',
      }),
    ],
    circles: [
      create(MapCircleSchema, {
        label: 'Water reach',
        center: create(GeoPointSchema, { lat: 41.1, lon: -72.201 }),
        radiusMeters: 18,
        strokeColor: '#047857',
      }),
    ],
  });
}

function renderWithClient(response: Promise<unknown>) {
  return html`
    <wiki-map
      page="garden_plan"
      name="yard"
      ${ref((element?: Element) => {
        if (!element) return;
        const mapElement = element as WikiMap;
        Object.defineProperty(mapElement, 'client', {
          value: { getMap: () => response },
          configurable: true,
        });
        mapElement.rendererFactory = () => new StoryRenderer();
      })}
    ></wiki-map>
  `;
}

export const MarkerPolygonCircle: Story = {
  render: () => renderWithClient(Promise.resolve(create(GetMapResponseSchema, { map: sampleMap() }))),
  parameters: {
    docs: {
      description: {
        story: 'Shows a map containing markers, a polygon, and a circle. Open the browser developer tools console to see the action logs.',
      },
    },
  },
};

export const Loading: Story = {
  render: () => renderWithClient(new Promise(() => {})),
};

export const ErrorState: Story = {
  render: () => renderWithClient(Promise.reject(new Error('MapService unavailable'))),
};
