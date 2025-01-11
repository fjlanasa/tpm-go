import type { Route } from "./+types/home";
import { TripsLayer } from '@deck.gl/geo-layers';
import { Map as MapLibreMap } from 'react-map-gl/maplibre';
import DeckGL from "deck.gl";
import { useEffect, useState } from "react";

export function meta({ }: Route.MetaArgs) {
  return [
    { title: "New React Router App" },
    { name: "description", content: "Welcome to React Router!" },
  ];
}

type Trip = {
  coordinates: number[][];
  timestamps: number[];
  route_id: string;
}

type TripsHandlerGenerator = AsyncGenerator<Trip[], void, unknown>;

class TripsClient {
  private eventSource: EventSource;
  private tripMap: Map<string, Trip>;

  constructor() {
    this.eventSource = new EventSource('http://localhost:8080/events?event_type=vehicle-position');
    this.tripMap = new Map<string, Trip>();
    this.eventSource.onmessage = (event) => {
      const data = JSON.parse(event.data);
      const tripId = data.trip?.tripId || data.vehicle?.id || 'unknown';
      // Get existing trip or create new one
      const existingTrip: Trip = this.tripMap.get(tripId) || {
        coordinates: [],
        timestamps: [],
        route_id: data.trip?.routeId || 'unknown'
      };

      // Add new point and timestamp
      existingTrip.coordinates.push([data.position.longitude, data.position.latitude]);
      existingTrip.timestamps.push(data.timestamp);

      // Update the map
      this.tripMap.set(tripId, existingTrip);
    };
  }

  async * getTripsGenerator(): TripsHandlerGenerator {
    while (true) {
      if (this.tripMap.size > 0) {
        const currentTrips = Array.from(this.tripMap.values());
        yield currentTrips;
      }
      await new Promise(resolve => setTimeout(resolve, 100));
    }
  }
}

export default function Home() {
  const [tripsGenerator, setTripsGenerator] = useState<TripsClient | null>(null);

  useEffect(() => {
    if (typeof window !== 'undefined') {
      const tripsGenerator = new TripsClient();
      setTripsGenerator(tripsGenerator);
    }
  }, []);

  const [currentTime, setCurrentTime] = useState(Date.now());

  useEffect(() => {
    const interval = setInterval(() => {
      const oneSecondAgo = new Date(Date.now() - 1000);
      console.log(oneSecondAgo);
      setCurrentTime(Math.floor(oneSecondAgo.getTime() / 1000));
    }, 1000);
    return () => clearInterval(interval);
  }, []);

  console.log(currentTime);

  const layer = new TripsLayer({
    id: 'TripsLayer',
    data: tripsGenerator?.getTripsGenerator(),
    getPath: d => d.coordinates,
    getTimestamps: d => d.timestamps,
    currentTime: currentTime,
    getColor: (d) => {
      if (d.route_id === 'Red') {
        return [253, 128, 93];
      } else if (d.route_id.includes('Green')) {
        return [0, 128, 0];
      } else if (d.route_id === 'Blue') {
        return [0, 0, 255];
      } else if (d.route_id === 'Orange') {
        return [255, 165, 0];
      } else if (d.route_id.includes('CR')) {
        return [148, 0, 211];
      }
      return [255, 215, 0];
    },
    trailLength: 600,
    capRounded: true,
    jointRounded: true,
    widthMinPixels: 8
  });

  return (
    <DeckGL
      initialViewState={{
        longitude: -71.06,
        latitude: 42.36,
        zoom: 11
      }}
      controller
      layers={[layer]}
    >
      <MapLibreMap mapStyle="https://basemaps.cartocdn.com/gl/positron-gl-style/style.json" />
    </DeckGL>
  )
}
