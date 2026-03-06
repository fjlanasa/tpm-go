-- PostgreSQL migration: create transit metrics event tables

CREATE TABLE IF NOT EXISTS stop_events (
    id                   BIGSERIAL PRIMARY KEY,
    agency_id            TEXT NOT NULL DEFAULT '',
    vehicle_id           TEXT NOT NULL DEFAULT '',
    route_id             TEXT NOT NULL DEFAULT '',
    stop_id              TEXT NOT NULL DEFAULT '',
    origin_stop_id       TEXT NOT NULL DEFAULT '',
    destination_stop_id  TEXT NOT NULL DEFAULT '',
    direction_id         TEXT NOT NULL DEFAULT '',
    direction            TEXT NOT NULL DEFAULT '',
    direction_destination TEXT NOT NULL DEFAULT '',
    parent_station       TEXT NOT NULL DEFAULT '',
    stop_sequence        INTEGER NOT NULL DEFAULT 0,
    stop_status          INTEGER NOT NULL DEFAULT 0,
    trip_id              TEXT NOT NULL DEFAULT '',
    service_date         TEXT NOT NULL DEFAULT '',
    event_timestamp      TIMESTAMPTZ,
    stop_event_type      TEXT NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_stop_events_agency_route_date
    ON stop_events (agency_id, route_id, service_date);

CREATE INDEX IF NOT EXISTS idx_stop_events_trip_stop
    ON stop_events (trip_id, stop_id);

-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS dwell_time_events (
    id                   BIGSERIAL PRIMARY KEY,
    agency_id            TEXT NOT NULL DEFAULT '',
    vehicle_id           TEXT NOT NULL DEFAULT '',
    route_id             TEXT NOT NULL DEFAULT '',
    stop_id              TEXT NOT NULL DEFAULT '',
    origin_stop_id       TEXT NOT NULL DEFAULT '',
    destination_stop_id  TEXT NOT NULL DEFAULT '',
    direction_id         TEXT NOT NULL DEFAULT '',
    direction            TEXT NOT NULL DEFAULT '',
    direction_destination TEXT NOT NULL DEFAULT '',
    parent_station       TEXT NOT NULL DEFAULT '',
    stop_sequence        INTEGER NOT NULL DEFAULT 0,
    stop_status          INTEGER NOT NULL DEFAULT 0,
    trip_id              TEXT NOT NULL DEFAULT '',
    service_date         TEXT NOT NULL DEFAULT '',
    event_timestamp      TIMESTAMPTZ,
    arrival_time         TIMESTAMPTZ,
    departure_time       TIMESTAMPTZ,
    dwell_time_seconds   INTEGER NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dwell_time_events_agency_route_date
    ON dwell_time_events (agency_id, route_id, service_date);

-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS travel_time_events (
    id                   BIGSERIAL PRIMARY KEY,
    agency_id            TEXT NOT NULL DEFAULT '',
    vehicle_id           TEXT NOT NULL DEFAULT '',
    route_id             TEXT NOT NULL DEFAULT '',
    stop_id              TEXT NOT NULL DEFAULT '',
    origin_stop_id       TEXT NOT NULL DEFAULT '',
    destination_stop_id  TEXT NOT NULL DEFAULT '',
    direction_id         TEXT NOT NULL DEFAULT '',
    direction            TEXT NOT NULL DEFAULT '',
    direction_destination TEXT NOT NULL DEFAULT '',
    parent_station       TEXT NOT NULL DEFAULT '',
    stop_sequence        INTEGER NOT NULL DEFAULT 0,
    stop_status          INTEGER NOT NULL DEFAULT 0,
    trip_id              TEXT NOT NULL DEFAULT '',
    service_date         TEXT NOT NULL DEFAULT '',
    event_timestamp      TIMESTAMPTZ,
    start_time           TIMESTAMPTZ,
    end_time             TIMESTAMPTZ,
    travel_time_seconds  INTEGER NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_travel_time_events_agency_route_date
    ON travel_time_events (agency_id, route_id, service_date);

CREATE INDEX IF NOT EXISTS idx_travel_time_events_origin_dest
    ON travel_time_events (origin_stop_id, destination_stop_id);

-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS headway_time_events (
    id                   BIGSERIAL PRIMARY KEY,
    agency_id            TEXT NOT NULL DEFAULT '',
    vehicle_id           TEXT NOT NULL DEFAULT '',
    route_id             TEXT NOT NULL DEFAULT '',
    stop_id              TEXT NOT NULL DEFAULT '',
    origin_stop_id       TEXT NOT NULL DEFAULT '',
    destination_stop_id  TEXT NOT NULL DEFAULT '',
    direction_id         TEXT NOT NULL DEFAULT '',
    direction            TEXT NOT NULL DEFAULT '',
    direction_destination TEXT NOT NULL DEFAULT '',
    parent_station       TEXT NOT NULL DEFAULT '',
    stop_sequence        INTEGER NOT NULL DEFAULT 0,
    stop_status          INTEGER NOT NULL DEFAULT 0,
    trip_id              TEXT NOT NULL DEFAULT '',
    service_date         TEXT NOT NULL DEFAULT '',
    event_timestamp      TIMESTAMPTZ,
    leading_vehicle_id   TEXT NOT NULL DEFAULT '',
    following_vehicle_id TEXT NOT NULL DEFAULT '',
    headway_seconds      INTEGER NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_headway_time_events_agency_route_date
    ON headway_time_events (agency_id, route_id, service_date);
