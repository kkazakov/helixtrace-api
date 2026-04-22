# REST API Reference

**Type:** Reference

Complete endpoint reference for helixtrace-api. All endpoints return JSON. Protected endpoints require `Authorization: Bearer <token>` header.

## Authentication

### POST /api/login

Authenticate user and receive a bearer token.

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "secret"
}
```

**Response 200:**
```json
{
  "token": "64-char-hex-string",
  "email": "user@example.com",
  "username": "user@example.com"
}
```

**Error Responses:**
- `400` — Missing email/password, invalid JSON
- `401` — Invalid email or password
- `403` — Account is disabled

---

### POST /api/register

Create a new user account.

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "secret"
}
```

**Response 201:**
```json
{
  "token": "64-char-hex-string",
  "email": "user@example.com",
  "username": "user@example.com"
}
```

**Error Responses:**
- `400` — Missing email/password, invalid JSON
- `409` — User already exists

---

## Health

### GET /api/health

Health check endpoint. No authentication required.

**Response 200:**
```json
{
  "status": "ok"
}
```

---

## Profile

### GET /api/profile

**Auth:** Required

Get authenticated user's email.

**Response 200:**
```json
{
  "email": "user@example.com"
}
```

---

## Trace Path

### GET /api/trace-path

**Auth:** Required

Get elevation profile between two coordinates. Results are cached in ClickHouse.

**Query Parameters:**
| Parameter | Required | Format | Description |
|---|---|---|---|
| `from` | Yes | `lat,lon` | Start coordinates |
| `to` | Yes | `lat,lon` | End coordinates |

**Example:**
```
GET /api/trace-path?from=42.4233664,23.0068696&to=42.4817369,23.0368781
```

**Response 200:**
```json
{
  "points": [
    {"lat": 42.4233664, "lng": 23.0068696, "elv": 500.0},
    {"lat": 42.4240000, "lng": 23.0075000, "elv": 502.5}
  ],
  "count": 150,
  "distance_between_points": 50.0,
  "status": "ok"
}
```

**Error Responses:**
- `400` — Missing/invalid from/to parameters
- `502` — Elevation service error

---

## Points

### POST /api/point

**Auth:** Required

Create a new geographic point. Elevation is fetched automatically from OpenTopoData.

**Request Body:**
```json
{
  "lat": 42.6977,
  "lon": 23.3219,
  "public": false,
  "label": "Sofia office",
  "category_id": 1
}
```

**Response 201:**
```json
{
  "id": "f6271fdd-95ce-4520-81da-eecac0f6039d",
  "lat": 42.6977,
  "lon": 23.3219,
  "elevation": 550.5,
  "public": false,
  "label": "Sofia office",
  "category_id": 1
}
```

**Error Responses:**
- `400` — Invalid coordinates, missing category_id, invalid category
- `502` — Elevation fetch failed

---

### GET /api/point/{id}

**Auth:** Required

Get point details. Returns public points from other users; only the owner sees the `user` field.

**Response 200:**
```json
{
  "data": {
    "id": "f6271fdd-95ce-4520-81da-eecac0f6039d",
    "lat": 42.661232,
    "lon": 23.147163,
    "elevation": 948.84,
    "public": true,
    "label": "Mountain peak",
    "category_id": 1,
    "user": "owner@example.com"
  }
}
```

**Error Responses:**
- `404` — Point not found, deleted, or not accessible

---

### PUT /api/point/{id}

**Auth:** Required

Update a point. Only provided fields are changed; omitted fields retain current values. Owner-only operation.

**Request Body:**
```json
{
  "label": "Updated label",
  "public": true
}
```

**Response 200:**
```json
{
  "id": "f6271fdd-95ce-4520-81da-eecac0f6039d",
  "lat": 42.661232,
  "lon": 23.147163,
  "elevation": 948.84,
  "public": true,
  "label": "Updated label",
  "category_id": 1
}
```

**Error Responses:**
- `400` — Invalid coordinates, invalid category_id
- `404` — Point not found or not owned by user

---

### DELETE /api/point/{id}

**Auth:** Required

Soft-delete a point. Owner-only operation.

**Response 200:**
```json
{
  "status": "deleted"
}
```

**Error Responses:**
- `404` — Point not found or not owned by user

---

### GET /api/points

**Auth:** Required

List points. By default returns only the authenticated user's points.

**Query Parameters:**
| Parameter | Type | Default | Description |
|---|---|---|---|
| `include_public` | boolean | `false` | Include other users' public points |
| `include_meshcore_dashboard` | boolean | `false` | Include external Meshcore repeaters |

**Response 200:**
```json
[
  {
    "id": "f6271fdd-95ce-4520-81da-eecac0f6039d",
    "lat": 42.661232,
    "lon": 23.147163,
    "elevation": 948.84,
    "public": true,
    "label": "Mountain peak",
    "category_id": 1
  }
]
```

When `include_meshcore_dashboard=true`, external repeater entries include `"external": true`.

---

### GET /api/point/info

**Auth:** Required

Fetch elevation for arbitrary coordinates.

**Query Parameters:**
| Parameter | Required | Description |
|---|---|---|
| `lat` | Yes | Latitude (-90 to 90) |
| `lon` | Yes | Longitude (-180 to 180) |

**Response 200:**
```json
{
  "data": {
    "lat": 42.6977,
    "lon": 23.3219,
    "elevation": 550.5
  }
}
```

---

### GET /api/point-categories

**Auth:** Required

List available point categories.

**Response 200:**
```json
[
  {"id": 1, "name": "poi"},
  {"id": 2, "name": "repeater"},
  {"id": 3, "name": "unknown"}
]
```

---

## Error Response Format

All error responses use a consistent format:

```json
{
  "error": "descriptive error message"
}
```
