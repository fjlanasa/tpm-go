# TPM-Go

A real-time transit performance monitoring system written in Go. This system processes GTFS-RT vehicle position feeds to generate various transit performance metrics including:

- Stop Events (arrivals/departures)
- Headway Times (time between trips at a stop for a given route)
- Dwell Times (time spent at stops for a given trip)
- Travel Times (time between stops for a given trip)

## Prerequisites

- Go 1.21 or later
- Protocol Buffers compiler (`protoc`)
- `protoc-gen-go` plugin

### Installing Prerequisites
