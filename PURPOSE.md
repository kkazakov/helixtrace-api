# Purpose

## Problem Statement

Field operators and radio enthusiasts need elevation profile data along routes between geographic coordinates, and a way to mark and manage points of interest (POIs) and repeater locations. Existing solutions either lack caching for repeated queries, don't support user-managed points, or require multiple services.

## Target Users & Stakeholders

- **Field operators** — Plan routes and check elevation profiles for radio communication planning
- **Radio enthusiasts** — Track repeater locations and signal coverage areas
- **Internal engineering** — Maintain and extend the API service

## Value & Success Metrics

- Reduced latency for repeated trace path queries via ClickHouse caching
- Single service for elevation data and point management
- Configurable elevation data source for operational flexibility
- User-managed points with sharing capabilities (public/private)

## Non-Goals

- Real-time tracking or live position updates
- Offline elevation data download
- Multi-tenant isolation beyond user-scoped data
- Advanced RBAC (access_rights column exists but is unused)
- Geodesic-accurate path interpolation (linear interpolation is sufficient for target distances)

## Constraints

- ClickHouse as the sole database (no PostgreSQL, no Redis)
- OpenTopoData-compatible API for elevation data
- Bearer token authentication (no OAuth, no session cookies)
- Stateless API server (all state in ClickHouse)

## System Role

Helixtrace API is a backend service that provides elevation data and point management to client applications. It is not a standalone product — it serves a web frontend or mobile application. See [Architecture Overview](docs/explanation/architecture-overview.md) for system context.

## Lifecycle Expectations

- API server runs continuously with `restart: unless-stopped` policy
- ClickHouse persists data across restarts via Docker volume
- Migrations are idempotent and re-run on every startup
- Token cleanup is handled by ClickHouse TTL (eventual, not immediate)
