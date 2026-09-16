# Youthpreneur API Contract

Base path: root (`/`). Routes are registered without a version prefix.

Content type: `application/json`. Request bodies are capped at 1 MiB.

Authentication: none yet. No authentication or authorization middleware is
wired, so every endpoint is public. See the safety limitation below before
exposing this service.

## Common responses

Errors return an HTTP status with a JSON error body:

```json
{"error": "message"}
```

| Status | Meaning |
| --- | --- |
| `400` | Malformed request body, cursor, limit, or field validation failure |
| `401` | Current password does not match |
| `404` | No active user matches the public ID |
| `409` | Email already registered |
| `500` | Unexpected server or database failure |

Timestamps `created_at` and `updated_at` are Unix epoch seconds as integers.
`birth_date` is exchanged as an ISO 8601 date (`YYYY-MM-DD`). Absent profile
fields are omitted from responses.

## Health

| Method | Path | Auth | Response |
| --- | --- | --- | --- |
| `GET` | `/healthz` | Public | `{"status":"ok"}` |

## Users

Public IDs use the form `YTP-` plus six random decimal digits, for example
`YTP-482910`. They are lookup handles, not authentication credentials, and must
not be treated as secret. Passwords are hashed with bcrypt and are never
returned in responses. Passwords must be 8 to 72 bytes.

| Method | Path | Auth | Request |
| --- | --- | --- | --- |
| `POST` | `/users` | Public | `CreateUserRequest` JSON |
| `GET` | `/users` | Public | Query: `cursor`, `limit`, `district`, `gender` |
| `GET` | `/users/{publicID}` | Public | Public ID path parameter |
| `DELETE` | `/users/{publicID}` | Public | Public ID path parameter |
| `PUT` | `/users/{publicID}/profile` | Public | `UpdateProfileRequest` JSON |
| `PATCH` | `/users/{publicID}/status` | Public | `UpdateStatusRequest` JSON |
| `PUT` | `/users/{publicID}/password` | Public | `ChangePasswordRequest` JSON |
| `POST` | `/users/{publicID}/password/reset` | Public | `ResetPasswordRequest` JSON |

Request fields:

- `CreateUserRequest`: `email`, `password`, `role` (accepted but ignored, always
  stored as `member`), `full_name`, `nik`, `birth_date`, `gender`, `district`,
  `phone`, `address`.
- `UpdateProfileRequest`: `full_name`, `nik`, `birth_date`, `gender`,
  `district`, `phone`, `address`. Omitted fields are cleared.
- `UpdateStatusRequest`: `is_active` (boolean, required).
- `ChangePasswordRequest`: `current_password`, `new_password`.
- `ResetPasswordRequest`: `new_password`.

User response fields: `public_id`, `email`, `role`, `is_active`, `created_at`,
`updated_at`, and optional `profile`. The profile contains `full_name`, `nik`,
`birth_date`, `gender`, `district`, `phone`, and `address`. The password hash is
never serialized.

`DELETE /users/{publicID}` and both password endpoints return `204 No Content`.
`POST /users` returns `201 Created`; the remaining endpoints return `200 OK`.

Example list response:

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
        "full_name": "Alice",
        "district": "Bandung",
        "phone": "08123456789",
        "address": "Jalan Mawar 1"
      }
    }
  ],
  "next_cursor": "42"
}
```

## Cursor pagination

`GET /users` is ordered by `users.id` ascending and never uses offset
pagination.

| Query parameter | Description |
| --- | --- |
| `cursor` | Return users with `id` greater than this value |
| `limit` | Page size, clamped to 1-100; default 20 |
| `district` | Optional exact profile district filter |
| `gender` | Optional `male` or `female` filter |

The cursor is the string form of the last seen `users.id`. Pass the returned
`next_cursor` back as `cursor`. The field is omitted on the last page. A
malformed cursor or non-positive limit is rejected with `400`.

## Safety limitations

There is no authentication or authorization yet. In particular,
`POST /users/{publicID}/password/reset` resets a password without the current
one. This endpoint is unsafe until authentication middleware is added; then
restrict it to authenticated admins only. Do not expose it outside a trusted
development network before that.

New users are always created with the `member` role. Any `role` sent to
`POST /users` is ignored. Admin provisioning needs a trusted, authenticated
path, which this service does not yet provide.

## Compatibility rules

- JSON field names are defined by tags in `internal/model`.
- Clients must treat list ordering as server-defined and use `next_cursor`.
- Clients must not construct cursors; return the received value unchanged.
- Authentication and authorization rules are enforced server-side; hiding a
  route in a client does not grant permission.

## Source of truth

Route registration: `internal/delivery/http/route/route.go`.

Endpoint handlers: `internal/delivery/http/handler/*_handler.go`.

Request and response models: `internal/model`.
