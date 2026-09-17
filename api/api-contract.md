# Youthpreneur API Contract

## Authentication

Authentication and authorization middleware are not implemented yet. All routes
are currently public. Protected behavior must be added before exposing this API
outside a trusted development environment.

When authentication is added, protected endpoints must use:

```text
Authorization: Bearer {token}
```

---

## Health Check Endpoints

### 1. Liveness Check

Check whether the service is running.

**Endpoint:** `GET /healthz`

**Response:**

```json
{
  "status": "ok"
}
```

**Status Code:** `200 OK`

---

## User Endpoints

### 2. Create User

Create a user and its profile.

**Endpoint:** `POST /users`

**Authentication:** Public for now. Authentication is required before production use.

**Content-Type:** `application/json`

**Request Fields:**

| Field | Type | Required | Format | Notes |
| --- | --- | --- | --- | --- |
| `email` | string | ✓ | Valid email | Unique user email |
| `password` | string | ✓ | 8-72 bytes | Stored as bcrypt hash; never returned |
| `full_name` | string | ✓ | — | Profile full name |
| `nik` | string | No | — | National identity number |
| `birth_date` | string | No | `YYYY-MM-DD` | Cannot be in the future |
| `gender` | string | No | `male` or `female` | Profile gender |
| `district` | string | No | — | Profile district filter field |
| `phone` | string | No | — | Contact phone number |
| `address` | string | No | — | Profile address |

**Request Example:**

```json
{
  "email": "alice@example.com",
  "password": "password123",
  "full_name": "Alice Example",
  "nik": "3273010101010001",
  "birth_date": "1995-05-20",
  "gender": "female",
  "district": "Bandung",
  "phone": "08123456789",
  "address": "Jalan Mawar 1"
}
```

New users always receive the `member` role. Admin provisioning requires a
trusted authenticated admin flow. Passwords must contain 8 to 72 bytes. `birth_date` must use
`YYYY-MM-DD` and cannot be in the future.

**Response:**

```json
{
  "public_id": "YTP-482910",
  "email": "alice@example.com",
  "role": "member",
  "is_active": true,
  "created_at": 1700000000,
  "updated_at": 1700000000,
  "profile": {
    "full_name": "Alice Example",
    "nik": "3273010101010001",
    "birth_date": "1995-05-20",
    "gender": "female",
    "district": "Bandung",
    "phone": "08123456789",
    "address": "Jalan Mawar 1"
  }
}
```

**Status Code:** `201 Created`

**Errors:** `400 Bad Request`, `409 Conflict`, `500 Internal Server Error`

---

### 3. List Users

List active users with optional profile filters and cursor pagination.

**Endpoint:** `GET /users`

**Authentication:** Public for now.

**Query Parameters:**

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `cursor` | integer string | No | Return users with `users.id` greater than cursor |
| `limit` | integer | No | Page size, default 20, maximum 100 |
| `district` | string | No | Exact profile district filter |
| `gender` | string | No | `male` or `female` |

**Response:**

```json
{
  "users": [
    {
      "public_id": "YTP-482910",
      "email": "alice@example.com",
      "role": "member",
      "is_active": true,
      "created_at": 1700000000,
      "updated_at": 1700000000,
      "profile": {
        "full_name": "Alice Example",
        "nik": "3273010101010001",
        "birth_date": "1995-05-20",
        "gender": "female",
        "district": "Bandung",
        "phone": "08123456789",
        "address": "Jalan Mawar 1"
      }
    }
  ],
  "next_cursor": "42"
}
```

`next_cursor` is omitted when no next page exists. Clients must pass the value
returned by the server and must not construct cursors.

**Status Code:** `200 OK`

**Errors:** `400 Bad Request`, `500 Internal Server Error`

---

### 4. Get User

Retrieve one active user and profile by public ID.

**Endpoint:** `GET /users/{publicID}`

**Authentication:** Public for now.

**Path Parameter:** `publicID` uses `YTP-` plus six random decimal digits.

**Response:**

```json
{
  "public_id": "YTP-482910",
  "email": "alice@example.com",
  "role": "member",
  "is_active": true,
  "created_at": 1700000000,
  "updated_at": 1700000000,
  "profile": {
    "full_name": "Alice Example",
    "nik": "3273010101010001",
    "birth_date": "1995-05-20",
    "gender": "female",
    "district": "Bandung",
    "phone": "08123456789",
    "address": "Jalan Mawar 1"
  }
}
```

**Status Code:** `200 OK`

**Errors:** `404 Not Found`, `500 Internal Server Error`

---

### 5. Soft Delete User

Soft delete a user and its profile in one database transaction.

**Endpoint:** `DELETE /users/{publicID}`

**Authentication:** Public for now; admin authorization is required before production use.

The server sets `deleted_at` and `updated_at` to the same Unix epoch-second value
on both `users` and `user_profiles`. Deleted users are excluded from reads.

**Status Code:** `204 No Content`

**Errors:** `404 Not Found`, `500 Internal Server Error`

---

### 6. Update User Profile

Replace profile fields for an active user. Omitted fields are cleared.

**Endpoint:** `PUT /users/{publicID}/profile`

**Authentication:** Public for now; authorization is required before production use.

**Request Fields:**

| Field | Type | Required | Format | Notes |
| --- | --- | --- | --- | --- |
| `full_name` | string | ✓ | — | Required profile full name |
| `nik` | string | No | — | Empty value clears field |
| `birth_date` | string | No | `YYYY-MM-DD` | Cannot be in the future |
| `gender` | string | No | `male` or `female` | Empty value clears field |
| `district` | string | No | — | Empty value clears field |
| `phone` | string | No | — | Empty value clears field |
| `address` | string | No | — | Empty value clears field |

**Request Example:**

```json
{
  "full_name": "Alice Example",
  "nik": "3273010101010001",
  "birth_date": "1995-05-20",
  "gender": "female",
  "district": "Bandung",
  "phone": "08123456789",
  "address": "Jalan Mawar 1"
}
```

**Response:**

```json
{
  "public_id": "YTP-482910",
  "email": "alice@example.com",
  "role": "member",
  "is_active": true,
  "created_at": 1700000000,
  "updated_at": 1700000001,
  "profile": {
    "full_name": "Alice Example",
    "nik": "3273010101010001",
    "birth_date": "1995-05-20",
    "gender": "female",
    "district": "Bandung",
    "phone": "08123456789",
    "address": "Jalan Mawar 1"
  }
}

```

**Status Code:** `200 OK`

**Errors:** `400 Bad Request`, `404 Not Found`, `500 Internal Server Error`

---

### 7. Update User Status

Set whether an active user account is active.

**Endpoint:** `PATCH /users/{publicID}/status`

**Authentication:** Public for now; authorization is required before production use.

**Request Fields:**

| Field | Type | Required | Format | Notes |
| --- | --- | --- | --- | --- |
| `is_active` | boolean | ✓ | `true` or `false` | Enables or disables the account |

**Request Example:**

```json
{
  "is_active": false
}
```

**Response:**

```json
{
  "public_id": "YTP-482910",
  "email": "alice@example.com",
  "role": "member",
  "is_active": false,
  "created_at": 1700000000,
  "updated_at": 1700000002,
  "profile": {
    "full_name": "Alice Example",
    "nik": "3273010101010001",
    "birth_date": "1995-05-20",
    "gender": "female",
    "district": "Bandung",
    "phone": "08123456789",
    "address": "Jalan Mawar 1"
  }
}
```

**Status Code:** `200 OK`

**Errors:** `400 Bad Request`, `404 Not Found`, `500 Internal Server Error`

---

### 8. Change Password

Change password when the current password is known.

**Endpoint:** `PUT /users/{publicID}/password`

**Authentication:** Public for now; authentication is required before production use.

**Request Fields:**

| Field | Type | Required | Format | Notes |
| --- | --- | --- | --- | --- |
| `current_password` | string | ✓ | 8-72 bytes | Must match stored password |
| `new_password` | string | ✓ | 8-72 bytes | Replacement password |

**Request Example:**

```json
{
  "current_password": "old-password",
  "new_password": "new-password"
}
```

The update is atomic against the current stored password hash. Passwords must be
8 to 72 bytes. The password is never returned.

**Status Code:** `204 No Content`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `404 Not Found`, `500 Internal Server Error`

---

### 9. Reset Password

Set a password without the current password. This is intended for an
authenticated administrator after auth middleware exists.

**Endpoint:** `POST /users/{publicID}/password/reset`

**Authentication:** Unsafe and public during current development stage. Must be
restricted to authenticated admins before production use.

**Request Fields:**

| Field | Type | Required | Format | Notes |
| --- | --- | --- | --- | --- |
| `new_password` | string | ✓ | 8-72 bytes | Replacement password; never returned |

**Request Example:**

```json
{
  "new_password": "new-password"
}
```

The server does not generate or return a password. Response has no body.

**Status Code:** `204 No Content`

**Errors:** `400 Bad Request`, `404 Not Found`, `500 Internal Server Error`

---

## Error Response Format

All errors use this shape:

```json
{
  "error": "message"
}
```

| Status | Meaning |
| --- | --- |
| `200 OK` | Request succeeded |
| `201 Created` | Resource created |
| `204 No Content` | Request succeeded without response body |
| `400 Bad Request` | Invalid JSON, query, or field value |
| `401 Unauthorized` | Current password is invalid |
| `404 Not Found` | Active user does not exist |
| `409 Conflict` | Email is already registered |
| `500 Internal Server Error` | Unexpected server or database failure |

Request bodies are limited to 1 MiB. Unknown JSON fields are currently ignored.

---

## Data and Time Format

- `created_at`, `updated_at`, and `deleted_at` use Unix epoch seconds as JSON integers.
- `deleted_at` is omitted from API responses and is NULL until soft deletion.
- `birth_date` uses ISO 8601 `YYYY-MM-DD`.
- User and profile primary keys are internal `SERIAL` integers.
- Public IDs use `YTP-DDDDDD`; they are lookup handles, not secrets or auth tokens.
- Password hashes are stored in `users.password` and never serialized.

---

## CORS and Rate Limiting

CORS and rate limiting are not implemented yet.

---

## Source of Truth

- Routes: `internal/delivery/http/route/route.go`
- Handlers: `internal/delivery/http/handler/*_handler.go`
- Models: `internal/model`
- API contract: `api/api-contract.md`
