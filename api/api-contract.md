# Youthpreneur API Contract

## Authentication

The API uses cookie-based JWT authentication.

- `POST /auth/login` verifies email and password, sets the auth cookies, and
  returns the authenticated user.
- `access_token` is a 15-minute HS256 JWT containing the user ID, public ID,
  role, issuer, issued-at, and expiry.
- `refresh_token` is an opaque 7-day value. Only its SHA-256 hash is stored in
  `refresh_sessions`; the raw value never reaches the database.
- Both cookies are `HttpOnly` and `SameSite=Lax`. `access_token` uses path `/`;
  `refresh_token` uses path `/auth`.
- Cookie `Secure` is `true` only when `APP_ENV=production`; local HTTP
  development uses `Secure=false`.
- `POST /auth/refresh` rotates the refresh token: the presented session is
  revoked and a replacement is issued in one transaction.
- Every login starts a new refresh token family (`refresh_sessions.family_id`).
  Rotation keeps the same family and stamps the consumed session with
  `revocation_reason = 'rotated'`, a 10-second `grace_until`, and the replacement
  token sealed with AES-256-GCM (key derived from `APP_AUTH_SECRET`). The raw
  replacement is never stored in plaintext and never logged. `APP_AUTH_SECRET`
  must remain stable across deployments: because the replacement is sealed with a
  key derived from it, rotating the secret while a session is inside its 10-second
  grace window makes that duplicate fail closed with `401` (never `500`), and the
  client must log in again.
- A duplicate `POST /auth/refresh` that presents the same old token within the
  10-second grace window is treated as a client retry: it returns `204` with the
  same replacement refresh token and a freshly issued access token. Repeated calls
  never extend `grace_until`. The grace path is read-only; if its stored
  replacement cannot be decrypted, the request fails with `401` like any other
  invalid refresh token.
- Reusing a rotated token after the grace window, or reusing a token revoked for
  any non-rotation reason, is a replay: it fails with `401` and revokes only that
  token's `family_id`. Other logins for the same user are unaffected.
- Password change, admin password reset, and account deactivation revoke every
  refresh session for the affected user with a non-rotation reason and clear the
  rotation grace metadata, so old tokens from those flows can never be redeemed
  through the grace window.
- Expired refresh sessions are deleted opportunistically during login and
  refresh; active sessions are never deleted.
- `POST /auth/logout` revokes the presented refresh token and clears both
  cookies.
- `GET /auth/me` returns the authenticated user's minimal identity from the
  current database row.

Protected requests authenticate with the `access_token` cookie; there is no
bearer header. Passwords and token values never appear in responses or logs.

### Route Permissions

| Route | Access |
| --- | --- |
| `GET /healthz` | Public |
| `POST /users` | Public |
| `POST /auth/login` | Public |
| `POST /auth/refresh` | Public, requires `refresh_token` cookie |
| `POST /auth/logout` | Authenticated |
| `GET /auth/me` | Authenticated |
| `PUT /users/{publicID}/profile` | Authenticated; owner or admin |
| `PUT /users/{publicID}/password` | Authenticated; owner or admin |
| `GET /users` | Admin |
| `GET /users/{publicID}` | Admin |
| `DELETE /users/{publicID}` | Admin |
| `PATCH /users/{publicID}/status` | Admin |
| `POST /users/{publicID}/password/reset` | Admin |
| `GET /enterprises/public` | Public |
| `POST /enterprises` | Authenticated; any member or admin |
| `GET /enterprises` | Authenticated; members see their own, admins see all |
| `GET /enterprises/{publicID}` | Authenticated; owner or admin |
| `GET /enterprises/{publicID}/audit-logs` | Authenticated; owner or admin |
| `PATCH /enterprises/{publicID}` | Authenticated; owner (limited fields) or admin |
| `DELETE /enterprises/{publicID}` | Authenticated; owner or admin |
| `GET /training-catalog` | Public |
| `GET /training-catalog/{publicID}` | Public |
| `POST /training-catalog` | Admin |
| `POST /training-catalog/upload-thumbnail` | Admin |
| `PATCH /training-catalog/{publicID}` | Admin |
| `PATCH /training-catalog/{publicID}/status` | Admin |
| `DELETE /training-catalog/{publicID}` | Admin |
| `POST /training-enrollments` | Authenticated |
| `PATCH /training-enrollments/{publicID}/status` | Admin |
| `DELETE /training-enrollments/{publicID}` | Authenticated; owner or admin |
| `GET /training-enrollments/my` | Authenticated; current user only |
| `GET /training-enrollments` | Admin |
| `GET /training-enrollments/catalog/{catalogPublicID}` | Admin |

Authentication rejects missing, invalid, or expired access tokens and inactive
or deleted users with `401`. Role and ownership checks reject authenticated but
unauthorized callers with `403`.

---

## Health Check Endpoints

### 1. Liveness Check

Check whether the service is running.

**Endpoint:** `GET /healthz`

**Authentication:** Public.

**Response:**

```json
{
  "status": "ok"
}
```

**Status Code:** `200 OK`

---

## Auth Endpoints

### 2. Login

Verify credentials, start a refresh session, and set auth cookies.

**Endpoint:** `POST /auth/login`

**Authentication:** Public. Rate limited by client IP; the default is 5 requests
per minute and `APP_RATE_LIMIT_LOGIN_PER_MINUTE` overrides it.

**Content-Type:** `application/json`

**Request Fields:**

| Field | Type | Required | Format | Notes |
| --- | --- | --- | --- | --- |
| `email` | string | ✓ | Valid email | Normalized to lowercase |
| `password` | string | ✓ | 8-72 bytes | Verified against the bcrypt hash |

**Request Example:**

```json
{
  "email": "alice@example.com",
  "password": "password123"
}
```

**Response:** The authenticated user. `access_token` and `refresh_token` are set
as `Set-Cookie` headers, never in the body.

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
    "district": "Bandung"
  }
}
```

**Status Code:** `200 OK`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `429 Too Many Requests`,
`500 Internal Server Error`

---

### 3. Refresh

Rotate the refresh token and issue a new access token.

**Endpoint:** `POST /auth/refresh`

**Authentication:** Public, but requires the `refresh_token` cookie. Rate
limited by client IP; the default is 10 requests per minute and
`APP_RATE_LIMIT_REFRESH_PER_MINUTE` overrides it.

**Request:** No body.

**Response:** No body. Both auth cookies are replaced.

Duplicate submissions inside the 10-second rotation grace window return the same
replacement refresh token with a new access token, so a retried request cannot
double-rotate. The grace deadline is fixed at the first rotation and is never
extended by later calls. Reuse outside the window is a replay that revokes only
the presented token's family and still returns `401`; other logins for the same
user keep working. Unknown, expired, or otherwise invalid tokens remain `401`
and cookie attributes are unchanged. A grace duplicate whose stored replacement
cannot be decrypted (for example after an `APP_AUTH_SECRET` rotation) is treated
as an invalid refresh token: `401`, not `500`. A duplicate that races a
concurrent rotation may lose the atomic rotate and also receive `401`.

**Status Code:** `204 No Content`

**Errors:** `401 Unauthorized`, `429 Too Many Requests`,
`500 Internal Server Error`

---

### 4. Logout

Revoke the presented refresh token and clear both auth cookies.

**Endpoint:** `POST /auth/logout`

**Authentication:** Authenticated (`access_token` cookie required).

**Request:** No body.

**Response:** No body. `access_token` and `refresh_token` are cleared.

**Status Code:** `204 No Content`

**Errors:** `401 Unauthorized`, `500 Internal Server Error`

---

### 5. Current User

Return the authenticated user's minimal identity and full name.

**Endpoint:** `GET /auth/me`

**Authentication:** Authenticated (`access_token` cookie required). The
response is derived from the current database row loaded by the authentication
middleware, so the role reflects the stored account role rather than the JWT
role claim.

**Request:** No body and no query parameters.

**Response:**

```json
{
  "public_id": "YTP-000123",
  "email": "user@example.com",
  "role": "member",
  "profile": {
    "full_name": "Jane Doe"
  }
}
```

`profile` is `null` when the user has no profile. The response deliberately
omits `is_active`, timestamps, password hashes, tokens, and other profile
fields.

**Status Code:** `200 OK`

**Errors:** `401 Unauthorized`, `500 Internal Server Error`

---

## User Endpoints

### 6. Create User

Create a user and its profile.

**Endpoint:** `POST /users`

**Authentication:** Public.

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

New users always receive the `member` role; the request cannot set a role. Admin
provisioning requires a trusted database or admin flow. Passwords must contain 8
to 72 bytes. `birth_date` must use `YYYY-MM-DD` and cannot be in the future.
`full_name` and `district` are trimmed and stored uppercased; other fields are
trimmed only.

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

**Errors:** `400 Bad Request`, `409 Conflict`, `429 Too Many Requests`,
`500 Internal Server Error`

---

### 7. List Users

List active member users with optional profile filters and cursor pagination.
Admin accounts are excluded. To retrieve a specific admin, use
`GET /users/{publicID}`.

**Endpoint:** `GET /users`

**Authentication:** Admin.

**Query Parameters:**

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `cursor` | integer string | No | Return users with `users.id` greater than cursor |
| `limit` | integer | No | Page size, default 20, maximum 100 |
| `district` | string | No | Exact profile district filter |
| `gender` | string | No | `male` or `female` |
| `search` | string | No | Case-insensitive partial `full_name` match (`ILIKE %search%`) |

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
returned by the server and must not construct cursors. Because admin accounts
are filtered before the cursor and limit are applied, the page size and cursor
stream cover member users only.

**Status Code:** `200 OK`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `403 Forbidden`,
`500 Internal Server Error`

---

### 8. Get User

Retrieve one active user and profile by public ID. Unlike `GET /users`, direct
lookup may return an active admin account.

**Endpoint:** `GET /users/{publicID}`

**Authentication:** Admin.

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

**Errors:** `401 Unauthorized`, `403 Forbidden`, `404 Not Found`,
`500 Internal Server Error`

---

### 9. Soft Delete User

Soft delete a user and its profile in one database transaction.

**Endpoint:** `DELETE /users/{publicID}`

**Authentication:** Admin.

The server sets `deleted_at` and `updated_at` to the same Unix epoch-second value
on both `users` and `user_profiles`. Deleted users are excluded from reads and
cannot authenticate.

**Status Code:** `204 No Content`

**Errors:** `401 Unauthorized`, `403 Forbidden`, `404 Not Found`,
`500 Internal Server Error`

---

### 10. Update User Profile

Replace profile fields for an active user. Omitted fields are cleared.

**Endpoint:** `PUT /users/{publicID}/profile`

**Authentication:** Authenticated; only the user in the path or an admin.

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

`full_name` and `district` are trimmed and stored uppercased; other fields are
trimmed only.

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

**Errors:** `400 Bad Request`, `401 Unauthorized`, `403 Forbidden`,
`404 Not Found`, `500 Internal Server Error`

---

### 11. Update User Status

Set whether an active user account is active.

**Endpoint:** `PATCH /users/{publicID}/status`

**Authentication:** Admin.

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
    "district": "Bandung"
  }
}
```

**Status Code:** `200 OK`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `403 Forbidden`,
`404 Not Found`, `500 Internal Server Error`

---

### 12. Change Password

Change password when the current password is known.

**Endpoint:** `PUT /users/{publicID}/password`

**Authentication:** Authenticated; only the user in the path or an admin. This
is self-service: changing your own password requires the current password. To set
another user's password, an admin uses `POST /users/{publicID}/password/reset`.

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
8 to 72 bytes. The password is never returned. A successful change revokes every
refresh session for the user.

**Status Code:** `204 No Content`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `403 Forbidden`,
`404 Not Found`, `500 Internal Server Error`

---

### 13. Reset Password

Set a password without the current password.

**Endpoint:** `POST /users/{publicID}/password/reset`

**Authentication:** Admin.

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

The server does not generate or return a password. Response has no body. A
successful reset revokes every refresh session for the user.

**Status Code:** `204 No Content`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `403 Forbidden`,
`404 Not Found`, `500 Internal Server Error`

---

## Enterprise Endpoints

Enterprises are independent aggregates linked one-to-many to users through
`enterprises.user_id`. Any authenticated member can create several enterprises;
the owner is always the authenticated user and `user_id` is never read from the
request body or query. Every create, update, and delete writes an audit row in
`enterprise_audit_events` in the same transaction as the mutation.

Enterprises use a generated `TPN-DDDDDD` public ID (`TPN-` plus six random digits),
which distinguishes them from user and training IDs (`YTP-DDDDDD`).

Enterprise business sectors (`business_sector_enum`): `Kuliner`,
`Perdagangan Ritel`, `Agribisnis & Ketahanan Pangan`,
`Jasa & Layanan Publik`, `Fashion & Konveksi`, `E-Commerce & Ekonomi Kreatif`.

Enterprise statuses (`enterprise_status_enum`): `active`, `inactive`.

Enterprise assessment enums:

- `legal_status_enum`: `complete`, `in_progress`, `none`
- `business_digitization_enum`: `high`, `medium`, `low`
- `intervention_needs_enum`: `Pelatihan`, `Mentoring`, `Digitalisasi`,
  `Legalitas`, `Permodalan`, `Kemitraan`, `Pemasaran`
- `training_status` and `mentoring_status` (`process_status_enum`):
  `completed`, `ongoing`, `planned`
- `capital_access` and `partnership` (`general_status_enum`): `yes`, `no`,
  `in_progress`

`enterprise_name` is required, trimmed, and cannot be empty (up to 255 characters).
`description` and `address` are optional `TEXT` fields.
`focus_commodity` and `dispora_support` are optional `VARCHAR(255)` fields.
The assessment fields are nullable and serialize as JSON `null` when unset.

`initial_turnover` and `current_turnover` are non-negative `DECIMAL(15,2)`
values serialized as JSON strings (for example `"1500.00"`) so precision is not
lost through a float round trip. Migration `000004` adds named `CHECK`
constraints that reject negative turnover and values at or above
`10000000000000` from any writer, not only this API. `district` is nullable and
serialized as JSON `null` when unset.

Enterprise list and get responses are enriched with owner details: `user_public_id`
(the owner's `YTP-DDDDDD` identifier) and `full_name` (from `user_profiles.full_name`,
or `null` if unset).

---

### 14. List Public Enterprises

List active enterprises for public consumption ordered by newest first with optional filters and cursor pagination. Does not expose turnover, audit history, or status.

**Endpoint:** `GET /enterprises/public`

**Authentication:** Public.

**Query Parameters:**

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `cursor` | integer string | No | Return enterprises with `enterprises.id` less than cursor (newest first) |
| `limit` | integer | No | Page size, default 9, maximum 100 |
| `search` | string | No | Case-insensitive match on `enterprise_name` or owner `full_name` (`search` or `q`) |
| `district` | string | No | Exact district filter |
| `intervention_needs` | string | No | Supported `intervention_needs_enum` value |
| `business_sector` | string | No | Supported `business_sector_enum` value |

**Response:**

```json
{
  "enterprises": [
    {
      "public_id": "TPN-482910",
      "enterprise_name": "Warung Kopi",
      "full_name": "Alice Example",
      "business_sector": "Kuliner",
      "district": "Bandung",
      "description": "Warung kopi tradisional dengan biji kopi lokal",
      "focus_commodity": "Kopi Robusta",
      "dispora_support": "Pelatihan Barista",
      "intervention_needs": "Permodalan",
      "created_at": 1700000000
    }
  ],
  "next_cursor": "42"
}
```

`next_cursor` is omitted when no next page exists.

**Status Code:** `200 OK`

**Errors:** `400 Bad Request`, `500 Internal Server Error`

---

### 15. Create Enterprise

Create an enterprise owned by the authenticated user.

**Endpoint:** `POST /enterprises`

**Authentication:** Authenticated. Any active member or admin.

**Content-Type:** `application/json`

**Request Fields:**

| Field | Type | Required | Format | Notes |
| --- | --- | --- | --- | --- |
| `enterprise_name` | string | ✓ | Up to 255 characters | Required; trimmed; cannot be empty |
| `business_sector` | string | ✓ | Supported `business_sector_enum` value | Required |
| `description` | string | No | Text | Optional; empty stores `null` |
| `address` | string | No | Text | Optional; empty stores `null` |
| `focus_commodity` | string | No | Up to 255 characters | Optional; empty stores `null` |
| `legal_status` | string | No | Supported `legal_status_enum` value | Empty stores `null` |
| `business_digitization` | string | No | Supported `business_digitization_enum` value | Empty stores `null` |
| `intervention_needs` | string | No | Supported `intervention_needs_enum` value | Empty stores `null` |
| `training_status` | string | No | Supported `process_status_enum` value | Empty stores `null` |
| `mentoring_status` | string | No | Supported `process_status_enum` value | Empty stores `null` |
| `capital_access` | string | No | Supported `general_status_enum` value | Empty stores `null` |
| `partnership` | string | No | Supported `general_status_enum` value | Empty stores `null` |
| `initial_turnover` | string | No | Non-negative `DECIMAL(15,2)` | Defaults to `"0.00"` |
| `current_turnover` | string | No | Non-negative `DECIMAL(15,2)` | Defaults to `"0.00"` |
| `district` | string | No | Up to 128 characters | Trimmed; empty stores `null` |

The owner is the authenticated user and the initial `status` is always
`active`; neither is accepted in the request.

**Request Example:**

```json
{
  "enterprise_name": "Warung Kopi",
  "business_sector": "Kuliner",
  "description": "Warung kopi tradisional dengan biji kopi lokal",
  "address": "Jl. Asia Afrika No. 10",
  "focus_commodity": "Kopi Robusta",
  "legal_status": "complete",
  "business_digitization": "high",
  "intervention_needs": "Permodalan",
  "training_status": "completed",
  "mentoring_status": "ongoing",
  "capital_access": "yes",
  "partnership": "no",
  "initial_turnover": "1500.00",
  "current_turnover": "1750.50",
  "district": "Bandung"
}
```

**Response:**

```json
{
  "public_id": "TPN-482910",
  "user_public_id": "YTP-482910",
  "full_name": "Alice Example",
  "enterprise_name": "Warung Kopi",
  "business_sector": "Kuliner",
  "description": "Warung kopi tradisional dengan biji kopi lokal",
  "address": "Jl. Asia Afrika No. 10",
  "focus_commodity": "Kopi Robusta",
  "dispora_support": null,
  "legal_status": "complete",
  "business_digitization": "high",
  "intervention_needs": "Permodalan",
  "training_status": "completed",
  "mentoring_status": "ongoing",
  "capital_access": "yes",
  "partnership": "no",
  "initial_turnover": "1500.00",
  "current_turnover": "1750.50",
  "district": "Bandung",
  "status": "active",
  "created_at": 1700000000,
  "updated_at": 1700000000
}
```

**Status Code:** `201 Created`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `429 Too Many Requests`,
`500 Internal Server Error`

---

### 16. List Enterprises

List active enterprises with optional filters and cursor pagination.

**Endpoint:** `GET /enterprises`

**Authentication:** Authenticated. Members are scoped to enterprises they own;
admins are not owner-restricted.

**Query Parameters:**

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `cursor` | integer string | No | Return enterprises with `enterprises.id` less than cursor (newest first) |
| `limit` | integer | No | Page size, default 20, maximum 100 |
| `search` | string | No | Case-insensitive match on `enterprise_name` or owner `full_name` (`search` or `q`) |
| `district` | string | No | Exact district filter |
| `status` | string | No | `active` or `inactive` |
| `business_sector` | string | No | Supported `business_sector_enum` value |
| `legal_status` | string | No | Supported `legal_status_enum` value |
| `business_digitization` | string | No | Supported `business_digitization_enum` value |
| `intervention_needs` | string | No | Supported `intervention_needs_enum` value |
| `training_status` | string | No | Supported `process_status_enum` value |
| `mentoring_status` | string | No | Supported `process_status_enum` value |
| `capital_access` | string | No | Supported `general_status_enum` value |
| `partnership` | string | No | Supported `general_status_enum` value |

Turnover values, IDs, and timestamps are intentionally not filterable. Results are ordered newest first (`ORDER BY e.id DESC`).

**Response:**

```json
{
  "enterprises": [
    {
      "public_id": "TPN-482910",
      "user_public_id": "YTP-482910",
      "full_name": "Alice Example",
      "enterprise_name": "Warung Kopi",
      "business_sector": "Kuliner",
      "description": "Warung kopi tradisional dengan biji kopi lokal",
      "address": "Jl. Asia Afrika No. 10",
      "focus_commodity": "Kopi Robusta",
      "dispora_support": "Pelatihan Barista",
      "legal_status": "complete",
      "business_digitization": "high",
      "intervention_needs": "Permodalan",
      "training_status": "completed",
      "mentoring_status": "ongoing",
      "capital_access": "yes",
      "partnership": "no",
      "initial_turnover": "1500.00",
      "current_turnover": "1750.50",
      "district": "Bandung",
      "status": "active",
      "created_at": 1700000000,
      "updated_at": 1700000000
    }
  ],
  "next_cursor": "42"
}
```

`next_cursor` is omitted when no next page exists. Clients must pass the value
returned by the server and must not construct cursors.

**Status Code:** `200 OK`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `500 Internal Server Error`

---

### 17. Get Enterprise

Retrieve one active enterprise by public ID.

**Endpoint:** `GET /enterprises/{publicID}`

**Authentication:** Authenticated; owner or admin. A member receives `404` for
an enterprise they do not own.

**Path Parameter:** `publicID` uses `TPN-` plus six random decimal digits (`TPN-DDDDDD`).

**Response:** The enterprise object shown in the list response.

**Status Code:** `200 OK`

**Errors:** `401 Unauthorized`, `404 Not Found`, `500 Internal Server Error`

---

### 18. Update Enterprise

Partially update an active enterprise. Omitted fields keep their current value.

**Endpoint:** `PATCH /enterprises/{publicID}`

**Authentication:** Authenticated; owner or admin.

**Request Fields:**

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `enterprise_name` | string | No | At most 255 characters; cannot be empty string |
| `business_sector` | string | No | Supported `business_sector_enum` value |
| `description` | string | No | Empty clears to `null` |
| `address` | string | No | Empty clears to `null` |
| `focus_commodity` | string | No | At most 255 characters; empty clears to `null` |
| `dispora_support` | string | No | Admin only; at most 255 characters; empty clears to `null` |
| `legal_status` | string | No | Admin only; supported `legal_status_enum` value |
| `business_digitization` | string | No | Admin only; supported `business_digitization_enum` value |
| `intervention_needs` | string | No | Admin only; supported `intervention_needs_enum` value |
| `training_status` | string | No | Admin only; supported `process_status_enum` value |
| `mentoring_status` | string | No | Admin only; supported `process_status_enum` value |
| `capital_access` | string | No | Admin only; supported `general_status_enum` value |
| `partnership` | string | No | Admin only; supported `general_status_enum` value |
| `initial_turnover` | string | No | Non-negative `DECIMAL(15,2)` |
| `current_turnover` | string | No | Non-negative `DECIMAL(15,2)` |
| `district` | string | No | Empty clears to `null` |
| `status` | string | No | Admin only; `active` or `inactive` |

Owners may change `enterprise_name`, `business_sector`, `district`, `description`,
`address`, `focus_commodity`, `initial_turnover`, and `current_turnover`.
An owner request that includes `dispora_support`, `status`, or any assessment field
is rejected with `403`. Admins may change any mutable field.
The response is the updated enterprise object.

**Status Code:** `200 OK`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `403 Forbidden`,
`404 Not Found`, `500 Internal Server Error`

---

### 19. Soft Delete Enterprise

Soft delete an active enterprise and record an audit event.

**Endpoint:** `DELETE /enterprises/{publicID}`

**Authentication:** Authenticated; owner or admin.

The server sets `deleted_at` and `updated_at` to the same Unix epoch-second
value. Deleted enterprises are excluded from reads and cannot be updated or
deleted again.

**Status Code:** `204 No Content`

**Errors:** `401 Unauthorized`, `403 Forbidden`, `404 Not Found`,
`500 Internal Server Error`

---

### 20. List Enterprise Audit Logs

List the audit event history (`create`, `update`, `delete`) for an enterprise, ordered newest first with cursor pagination.

**Endpoint:** `GET /enterprises/{publicID}/audit-logs`

**Authentication:** Authenticated; owner or admin. A member receives `404` for an enterprise they do not own.

**Path Parameter:** `publicID` uses `TPN-` plus six random decimal digits (`TPN-DDDDDD`).

**Query Parameters:**

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `cursor` | integer string | No | Return audit events with internal id less than cursor (newest first) |
| `limit` | integer | No | Page size, default 20, maximum 100 |

**Response:**

```json
{
  "events": [
    {
      "id": 15,
      "actor_public_id": "YTP-482910",
      "actor_email": "alice@example.com",
      "actor_name": "Alice Example",
      "action": "update",
      "changed_fields": {
        "current_turnover": "2000.00"
      },
      "created_at": 1700000050
    },
    {
      "id": 12,
      "actor_public_id": "YTP-482910",
      "actor_email": "alice@example.com",
      "actor_name": "Alice Example",
      "action": "create",
      "changed_fields": {
        "enterprise_name": "Warung Kopi"
      },
      "created_at": 1700000000
    }
  ],
  "next_cursor": "12"
}
```

`next_cursor` is omitted when no next page exists.

**Status Code:** `200 OK`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `404 Not Found`, `500 Internal Server Error`

---

## Training Catalog and Enrollment Endpoints

Training catalog entries are an independent aggregate with a generated
`YTP-DDDDDD` public ID. Catalog reads are public; catalog writes are admin-only.
Enrollments link one user to one catalog offering and always take
`user_id` from the authenticated identity, never from the request.

`training_status` reuses `process_status_enum`: `planned`, `ongoing`, or
`completed`. A catalog accepts enrollments only while its status is `planned` or
`ongoing`; `completed` or unset is closed. `training_slots` is an optional
positive capacity; `NULL` means unlimited. Migration `000005` adds a named
`CHECK` constraint (`training_catalog_training_slots_positive`) that rejects
non-positive capacity from any writer.

Cancellation is a soft delete of the enrollment (`deleted_at`), so history is
preserved. A partial unique index (`training_enrollments_active_unique`) allows
at most one active enrollment per user and catalog, and re-enrollment after
cancellation. Capacity is enforced transactionally: the enrollment insert locks
the catalog row with `SELECT ... FOR UPDATE`, then counts active enrollments so
`training_slots` is never exceeded, including under concurrent requests.

---

### 21. List Training Catalog

List active catalog entries with optional filters and cursor pagination.

**Endpoint:** `GET /training-catalog`

**Authentication:** Public.

**Query Parameters:**

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `cursor` | integer string | No | Return catalogs with `training_catalog.id` greater than cursor |
| `limit` | integer | No | Page size, default 20, maximum 100 |
| `category` | string | No | `training_category_enum` filter (`Wirausaha & Agribisnis`, `Kriya & Kreativitas`, `Digital & IPTEK`, `Olahraga & Prestasi`, `Komunitas & Pemuda`) |
| `training_status` | string | No | `planned`, `ongoing`, or `completed` |
| `start_date` | string | No | `YYYY-MM-DD` exact start date filter |

`title`, `pic_phone`, `mentor`, `address`, `thumbnail`, `end_date`, link, slots, IDs, and timestamps are not
filterable. `next_cursor` is omitted when no next page exists; clients must pass
the returned value and must not construct cursors.

**Response:**

```json
{
  "training_catalogs": [
    {
      "public_id": "YTP-482910",
      "title": "Bisnis Digital",
      "description": "Pelatihan pemasaran digital",
      "pic_phone": "08123456789",
      "category": "Wirausaha & Agribisnis",
      "max_slots": 30,
      "registered_count": 5,
      "training_status": "planned",
      "link": "https://example.com/training",
      "address": "Jl. Pemuda No. 1",
      "thumbnail": "/uploads/thumbnails/sample.png",
      "start_date": "2026-10-01",
      "end_date": "2026-10-05",
      "mentor": "Budi",
      "created_at": 1700000000,
      "updated_at": 1700000000
    }
  ],
  "next_cursor": "42"
}
```

**Status Code:** `200 OK`

**Errors:** `400 Bad Request`, `500 Internal Server Error`

---

### 22. Get Training Catalog

Retrieve one active catalog entry by public ID.

**Endpoint:** `GET /training-catalog/{publicID}`

**Authentication:** Public.

**Response:** The catalog object shown in the list response. Optional fields
serialize as JSON `null` when unset.

**Status Code:** `200 OK`

**Errors:** `404 Not Found`, `500 Internal Server Error`

---

### 23. Create Training Catalog

Create a catalog entry.

**Endpoint:** `POST /training-catalog`

**Authentication:** Admin.

**Request Fields:**

| Field | Type | Required | Format | Notes |
| --- | --- | --- | --- | --- |
| `title` | string | No | Up to 255 characters | Trimmed; empty stores `null` |
| `description` | string | No | — | Trimmed; empty stores `null` |
| `pic_phone` | string | No | Up to 50 characters | Trimmed; empty stores `null` |
| `category` | string | No | `training_category_enum` value | Exact category name; empty stores `null` |
| `max_slots` | integer | No | Positive | `null` or omitted means unlimited |
| `training_status` | string | No | `process_status_enum` value | Empty stores `null` |
| `link` | string | No | `http`/`https` URL, up to 255 characters | Empty stores `null` |
| `address` | string | No | — | Trimmed; empty stores `null` |
| `thumbnail` | string | No | Up to 255 characters | Trimmed; empty stores `null` |
| `start_date` | string | No | `YYYY-MM-DD` | Empty stores `null` |
| `end_date` | string | No | `YYYY-MM-DD` | Empty stores `null`; must be on or after `start_date` |
| `mentor` | string | No | Up to 255 characters | Trimmed; empty stores `null` |

`public_id`, timestamps, and deletion state are server-controlled.

**Status Code:** `201 Created`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `403 Forbidden`,
`500 Internal Server Error`

---

### 24. Upload Training Catalog Thumbnail

Upload an image to be used as a training catalog thumbnail.

**Endpoint:** `POST /training-catalog/upload-thumbnail`

**Authentication:** Admin.

**Request:** Multipart form (`multipart/form-data`) with a file field named `thumbnail`.
Only PNG, JPG, and JPEG images up to 5 MiB are accepted. The file is saved to the server's
upload directory and served statically under `/uploads/thumbnails/`.

**Response:**

```json
{
  "thumbnail_url": "/uploads/thumbnails/thumb_1700000000_abcdef1234567890.png"
}
```

**Status Code:** `201 Created`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `403 Forbidden`,
`500 Internal Server Error`

---

### 25. Update Training Catalog

Partially update an active catalog entry. Omitted fields keep their current
value; an empty string clears a nullable string field. `start_date` may be
changed but not cleared once set. A request that supplies no update fields is
rejected.

**Endpoint:** `PATCH /training-catalog/{publicID}`

**Authentication:** Admin.

**Request Fields:** The create fields as optional fields. `max_slots`, when
supplied, must be positive. `end_date` must be on or after `start_date`. At least
one field is required.

**Response:** The updated catalog object.

**Status Code:** `200 OK`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `403 Forbidden`,
`404 Not Found`, `500 Internal Server Error`

---

### 26. Update Training Catalog Status

Change only the training status of an active catalog entry.

**Endpoint:** `PATCH /training-catalog/{publicID}/status`

**Authentication:** Admin.

**Request Fields:**

| Field | Type | Required | Format | Notes |
| --- | --- | --- | --- | --- |
| `training_status` | string | ✓ | `planned`, `ongoing`, or `completed` | Required |

**Response:** The updated catalog object.

**Status Code:** `200 OK`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `403 Forbidden`,
`404 Not Found`, `500 Internal Server Error`

---

### 27. Soft Delete Training Catalog

Soft delete an active catalog entry.

**Endpoint:** `DELETE /training-catalog/{publicID}`

**Authentication:** Admin.

The server sets `deleted_at` and `updated_at` to the same Unix epoch-second
value. Deleted catalogs are excluded from reads. Cancelled and active
enrollments are retained.

**Status Code:** `204 No Content`

**Errors:** `401 Unauthorized`, `403 Forbidden`, `404 Not Found`,
`500 Internal Server Error`

---

### 28. Enroll in Training

Enroll the authenticated user in a catalog offering.

**Endpoint:** `POST /training-enrollments`

**Authentication:** Authenticated.

**Request Fields:**

| Field | Type | Required | Format | Notes |
| --- | --- | --- | --- | --- |
| `catalog_public_id` | string | ✓ | `YTP-DDDDDD` | Catalog to enroll in |

**Request Example:**

```json
{
  "catalog_public_id": "YTP-482910"
}
```

The enrolling user, `register_date` (current date), initial `status` (`pending`), and
timestamps are server-controlled. Enrollment is rejected with `409` when the catalog is
`completed`/unset, is full, or the user already has an active enrollment.
Enrollment is rejected with `404` when the catalog does not exist or is
soft deleted.

**Response:**

```json
{
  "public_id": "YTP-482920",
  "user_public_id": "YTP-000007",
  "register_date": "2026-09-17",
  "status": "pending",
  "created_at": 1700000000,
  "updated_at": 1700000000,
  "catalog": {
    "public_id": "YTP-482910",
    "title": "Bisnis Digital",
    "category": "Wirausaha & Agribisnis",
    "max_slots": 30,
    "registered_count": 5,
    "training_status": "planned"
  }
}
```

**Status Code:** `201 Created`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `404 Not Found`,
`409 Conflict`, `500 Internal Server Error`

---

### 29. Update Training Enrollment Status

Update an enrollment's approval status (`pending`, `accepted`, `rejected`).
When updated to `accepted`, the catalog's `registered_count` increases by 1.
If the catalog has reached `max_slots`, transition to `accepted` fails with `409 Conflict`.

**Endpoint:** `PATCH /training-enrollments/{publicID}/status`

**Authentication:** Admin.

**Request Fields:**

| Field | Type | Required | Format | Notes |
| --- | --- | --- | --- | --- |
| `status` | string | ✓ | `pending`, `accepted`, or `rejected` | Required |

**Request Example:**

```json
{
  "status": "accepted"
}
```

**Response:** The updated enrollment object.

**Status Code:** `200 OK`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `403 Forbidden`,
`404 Not Found`, `409 Conflict`, `500 Internal Server Error`

---

### 30. Cancel Training Enrollment

Cancel the caller's active enrollment.

**Endpoint:** `DELETE /training-enrollments/{publicID}`

**Authentication:** Authenticated; owner or admin. A member receives `404` for
an enrollment they do not own.

The server sets `deleted_at` and `updated_at` to the same Unix epoch-second
value. The row is retained so enrollment history survives, and the user may
enroll again afterward.

**Status Code:** `204 No Content`

**Errors:** `401 Unauthorized`, `404 Not Found`, `500 Internal Server Error`

---

### 31. My Training Enrollment History

List the current user's enrollment history, including cancelled enrollments.

**Endpoint:** `GET /training-enrollments/my`

**Authentication:** Authenticated.

**Query Parameters:** `cursor` and `limit` as described for catalog listing.

**Response:**

```json
{
  "training_enrollments": [
    {
      "public_id": "YTP-482920",
      "user_public_id": "YTP-000007",
      "register_date": "2026-09-17",
      "status": "accepted",
      "created_at": 1700000000,
      "updated_at": 1700000100,
      "catalog": {
        "public_id": "YTP-482910",
        "title": "Bisnis Digital",
        "training_status": "planned"
      }
    }
  ],
  "next_cursor": "42"
}
```

**Status Code:** `200 OK`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `500 Internal Server Error`

---

### 32. All Training Enrollment History

List every user's enrollment history, including cancelled enrollments.

**Endpoint:** `GET /training-enrollments`

**Authentication:** Admin.

**Query Parameters:** `cursor` and `limit` as described for catalog listing.

**Response:** The enrollment page shown for `/training-enrollments/my`.

**Status Code:** `200 OK`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `403 Forbidden`,
`500 Internal Server Error`

---

### 33. Catalog Training Enrollment History

List one catalog's enrollment history, including cancelled enrollments.

**Endpoint:** `GET /training-enrollments/catalog/{catalogPublicID}`

**Authentication:** Admin.

**Query Parameters:** `cursor` and `limit` as described for catalog listing.

**Response:** The enrollment page shown for `/training-enrollments/my`.

**Status Code:** `200 OK`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `403 Forbidden`,
`404 Not Found`, `500 Internal Server Error`

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
| `401 Unauthorized` | Missing, invalid, or expired credentials or refresh token |
| `403 Forbidden` | Authenticated but not permitted, or disallowed CORS origin |
| `404 Not Found` | Active user or enterprise does not exist, or is outside the caller's owner scope |
| `409 Conflict` | Email is registered, or training enrollment already exists, catalog is full, or catalog is closed |
| `429 Too Many Requests` | Rate limit exceeded; see `Retry-After` |
| `500 Internal Server Error` | Unexpected server or database failure |

Request bodies are limited to 1 MiB. Unknown JSON fields are currently ignored.

---

## Data and Time Format

- `created_at`, `updated_at`, and `deleted_at` use Unix epoch seconds as JSON integers.
- `deleted_at` is omitted from API responses and is NULL until soft deletion.
- `birth_date` uses ISO 8601 `YYYY-MM-DD`.
- User, profile, and enterprise primary keys are internal `SERIAL` integers.
- Public IDs use `YTP-DDDDDD` for users and training catalogs/enrollments, and `TPN-DDDDDD` for enterprises; they are lookup handles, not secrets or auth tokens.
- Password hashes are stored in `users.password` and never serialized.
- Refresh sessions store only SHA-256 token hashes plus expiry, revoke state, a
  `family_id`, a `revocation_reason`, and — only during the 10-second rotation
  grace window — a `grace_until` deadline and the AES-256-GCM sealed replacement
  token. Raw refresh tokens are never stored or logged. Security revocations
  clear the grace deadline and sealed replacement. `replaced_by_hash` is retained
  for schema compatibility but no longer drives replay decisions.
- Enterprise turnover uses `DECIMAL(15,2)` and is serialized as a JSON string.
- Enterprise ownership is one-to-many: one user owns many enterprises, and
  `enterprises.user_id` is always server-derived from the authenticated user.
- Enterprise `enterprise_name`, `business_sector`, and `status` are required, and `status` defaults to `active`.
  `description`, `address`, `focus_commodity`, `dispora_support`, `district`, and assessment enums are nullable.
- Enterprise mutations write an `enterprise_audit_events` row (actor, action,
  changed fields JSONB, timestamp) in the same transaction. Migration `000003`
  adds the enterprise tables, the `business_sector_enum`, `enterprise_status_enum`,
  `legal_status_enum`, `business_digitization_enum`, `intervention_needs_enum`,
  `process_status_enum`, and `general_status_enum` types, and indexes on
  `user_id`, `district`, `status`, `business_sector`, `legal_status`,
  `business_digitization`, `intervention_needs`, `training_status`,
  `mentoring_status`, `capital_access`, `partnership`, `deleted_at`, plus
  cursor-friendly and composite owner/deleted/id combinations.
- Migration `000007` renames `enterprises.name` to `enterprise_name` (NOT NULL),
  adds `description` (TEXT), `address` (TEXT), `focus_commodity` (VARCHAR(255)),
  `dispora_support` (VARCHAR(255)), and changes enterprise public ID generation
  and check constraint to `TPN-[0-9]{6}`.
- Training catalog optional fields are nullable; `max_slots` is a positive
  `INTEGER` or `NULL` for unlimited. Catalog `training_status` reuses
  `process_status_enum`; enrollment creation reserves capacity while the status
  is `planned` or `ongoing`.
- Enrollment `register_date` is server-derived from the current date. Initial enrollment
  status is `pending`; admins may update status to `accepted` or `rejected` via
  `PATCH /training-enrollments/{publicID}/status`.
- `registered_count` is dynamically calculated from active enrollments with `status = 'accepted'`.
  When an enrollment is updated to `accepted`, `registered_count` increments by 1.
- Enrollment cancellation is a soft delete that preserves history, and the
  active uniqueness index allows re-enrollment after cancellation.
- Thumbnail uploads accept PNG, JPG, and JPEG images up to 5 MiB, saved to
  `UPLOAD_DIR/thumbnails` and served via `GET /uploads/*`.
- Migration `000008` refactors `training_catalog` (renaming `name` to `title`,
  `speaker` to `mentor`, `training_slots` to `max_slots`), drops `training_date` and
  `training_period`, adds `address` (TEXT), `thumbnail` (VARCHAR(255)), `start_date` (DATE),
  `end_date` (DATE), creates `training_category_enum` (`'Wirausaha & Agribisnis'`,
  `'Kriya & Kreativitas'`, `'Digital & IPTEK'`, `'Olahraga & Prestasi'`, `'Komunitas & Pemuda'`),
  and adds `status` (`training_enrollment_status_enum`: `'pending'`, `'accepted'`, `'rejected'`, default `'pending'`)
  to `training_enrollments`.

---

## CORS and Rate Limiting

Credentialed CORS is enabled. Allowed origins come from
`APP_CORS_ALLOWED_ORIGINS` and default to
`http://localhost:3000,http://localhost:5173`. Preflight `OPTIONS` requests are
answered with the allowed methods and headers. A request carrying an origin that
is not configured is rejected with `403`.

Rate limiting is keyed by client IP in fixed one-minute windows and returns
`429` with a `Retry-After` header when exceeded. Rate limiting runs before CORS
so disallowed-origin requests are counted too.

| Bucket | Default | Override |
| --- | --- | --- |
| Login (`POST /auth/login`) | 5 per minute | `APP_RATE_LIMIT_LOGIN_PER_MINUTE` |
| Refresh (`POST /auth/refresh`) | 10 per minute | `APP_RATE_LIMIT_REFRESH_PER_MINUTE` |
| General API | 60 per minute | `APP_RATE_LIMIT_GENERAL_PER_MINUTE` |

The limiter uses the connection's `RemoteAddr` and in-memory state. It is only
correct for direct, single-process exposure. Behind a reverse proxy all clients
may share one address, and running multiple replicas multiplies the effective
limit. Add trusted-proxy client-IP handling and shared storage before proxied or
multi-replica deployments.

---

## Source of Truth

- Routes: `internal/delivery/http/route/route.go`
- Middleware: `internal/delivery/http/middleware/*.go`
- Handlers: `internal/delivery/http/handler/*_handler.go`
- Models: `internal/model`
- API contract: `api/api-contract.md`
