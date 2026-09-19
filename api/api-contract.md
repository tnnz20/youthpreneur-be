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
  revoked and a replacement is issued in one transaction. Reusing a rotated
  token fails with `401` and revokes every refresh session for that user.
- Password change, admin password reset, and account deactivation revoke every
  refresh session for the affected user.
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
| `POST /enterprises` | Authenticated; any member or admin |
| `GET /enterprises` | Authenticated; members see their own, admins see all |
| `GET /enterprises/{publicID}` | Authenticated; owner or admin |
| `PATCH /enterprises/{publicID}` | Authenticated; owner (limited fields) or admin |
| `DELETE /enterprises/{publicID}` | Authenticated; owner or admin |
| `GET /training-catalog` | Public |
| `GET /training-catalog/{publicID}` | Public |
| `POST /training-catalog` | Admin |
| `PATCH /training-catalog/{publicID}` | Admin |
| `PATCH /training-catalog/{publicID}/status` | Admin |
| `DELETE /training-catalog/{publicID}` | Admin |
| `POST /training-enrollments` | Authenticated |
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

`name` and the assessment fields are nullable and serialize as JSON `null` when
unset.

`initial_turnover` and `current_turnover` are non-negative `DECIMAL(15,2)`
values serialized as JSON strings (for example `"1500.00"`) so precision is not
lost through a float round trip. Migration `000004` adds named `CHECK`
constraints that reject negative turnover and values at or above
`10000000000000` from any writer, not only this API. `district` is nullable and
serialized as JSON `null` when unset.

---

### 14. Create Enterprise

Create an enterprise owned by the authenticated user.

**Endpoint:** `POST /enterprises`

**Authentication:** Authenticated. Any active member or admin.

**Content-Type:** `application/json`

**Request Fields:**

| Field | Type | Required | Format | Notes |
| --- | --- | --- | --- | --- |
| `name` | string | No | Up to 255 characters | Trimmed; empty stores `null` |
| `business_sector` | string | ✓ | Supported `business_sector_enum` value | Required |
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
  "name": "Warung Kopi",
  "business_sector": "Kuliner",
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
  "public_id": "YTP-482910",
  "name": "Warung Kopi",
  "business_sector": "Kuliner",
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

### 15. List Enterprises

List active enterprises with optional filters and cursor pagination.

**Endpoint:** `GET /enterprises`

**Authentication:** Authenticated. Members are scoped to enterprises they own;
admins are not owner-restricted.

**Query Parameters:**

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `cursor` | integer string | No | Return enterprises with `enterprises.id` greater than cursor |
| `limit` | integer | No | Page size, default 20, maximum 100 |
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

`name`, turnover values, IDs, and timestamps are intentionally not filterable.

**Response:**

```json
{
  "enterprises": [
    {
      "public_id": "YTP-482910",
      "name": "Warung Kopi",
      "business_sector": "Kuliner",
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

### 16. Get Enterprise

Retrieve one active enterprise by public ID.

**Endpoint:** `GET /enterprises/{publicID}`

**Authentication:** Authenticated; owner or admin. A member receives `404` for
an enterprise they do not own.

**Path Parameter:** `publicID` uses `YTP-` plus six random decimal digits.

**Response:** The enterprise object shown in the list response.

**Status Code:** `200 OK`

**Errors:** `401 Unauthorized`, `404 Not Found`, `500 Internal Server Error`

---

### 17. Update Enterprise

Partially update an active enterprise. Omitted fields keep their current value.

**Endpoint:** `PATCH /enterprises/{publicID}`

**Authentication:** Authenticated; owner or admin.

**Request Fields:**

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `name` | string | No | At most 255 characters; empty clears to `null` |
| `business_sector` | string | No | Supported `business_sector_enum` value |
| `legal_status` | string | No | Admin only; supported `legal_status_enum` value |
| `business_digitization` | string | No | Admin only; supported `business_digitization_enum` value |
| `intervention_needs` | string | No | Admin only; supported `intervention_needs_enum` value |
| `training_status` | string | No | Admin only; supported `process_status_enum` value |
| `mentoring_status` | string | No | Admin only; supported `process_status_enum` value |
| `capital_access` | string | No | Admin only; supported `general_status_enum` value |
| `partnership` | string | No | Admin only; supported `general_status_enum` value |
| `initial_turnover` | string | No | Non-negative `DECIMAL(15,2)` |
| `current_turnover` | string | No | Non-negative `DECIMAL(15,2)` |
| `district` | string | No | Admin only; empty clears to `null` |
| `status` | string | No | Admin only; `active` or `inactive` |

Owners may change only `name`, `business_sector`, `initial_turnover`, and
`current_turnover`. An owner request that includes `district`, `status`, or any
assessment field is rejected with `403`. Admins may change any mutable field.
The response is the updated enterprise object.

**Status Code:** `200 OK`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `403 Forbidden`,
`404 Not Found`, `500 Internal Server Error`

---

### 18. Soft Delete Enterprise

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

### 19. List Training Catalog

List active catalog entries with optional filters and cursor pagination.

**Endpoint:** `GET /training-catalog`

**Authentication:** Public.

**Query Parameters:**

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `cursor` | integer string | No | Return catalogs with `training_catalog.id` greater than cursor |
| `limit` | integer | No | Page size, default 20, maximum 100 |
| `category` | string | No | Exact category filter |
| `training_status` | string | No | `planned`, `ongoing`, or `completed` |
| `training_date` | string | No | `YYYY-MM-DD` exact date filter |
| `training_period` | string | No | Exact period filter |

`name`, `pic_phone`, `speaker`, link, slots, IDs, and timestamps are not
filterable. `next_cursor` is omitted when no next page exists; clients must pass
the returned value and must not construct cursors.

**Response:**

```json
{
  "training_catalogs": [
    {
      "public_id": "YTP-482910",
      "name": "Bisnis Digital",
      "description": "Pelatihan pemasaran digital",
      "pic_phone": "08123456789",
      "category": "Pemasaran",
      "training_slots": 30,
      "training_status": "planned",
      "link": "https://example.com/training",
      "training_date": "2026-10-01",
      "training_period": "09:00-12:00",
      "speaker": "Budi",
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

### 20. Get Training Catalog

Retrieve one active catalog entry by public ID.

**Endpoint:** `GET /training-catalog/{publicID}`

**Authentication:** Public.

**Response:** The catalog object shown in the list response. Optional fields
serialize as JSON `null` when unset.

**Status Code:** `200 OK`

**Errors:** `404 Not Found`, `500 Internal Server Error`

---

### 21. Create Training Catalog

Create a catalog entry.

**Endpoint:** `POST /training-catalog`

**Authentication:** Admin.

**Request Fields:**

| Field | Type | Required | Format | Notes |
| --- | --- | --- | --- | --- |
| `name` | string | No | Up to 255 characters | Trimmed; empty stores `null` |
| `description` | string | No | — | Trimmed; empty stores `null` |
| `pic_phone` | string | No | Up to 50 characters | Trimmed; empty stores `null` |
| `category` | string | No | Up to 100 characters | Trimmed; empty stores `null` |
| `training_slots` | integer | No | Positive | `null` or omitted means unlimited |
| `training_status` | string | No | `process_status_enum` value | Empty stores `null` |
| `link` | string | No | `http`/`https` URL, up to 255 characters | Empty stores `null` |
| `training_date` | string | No | `YYYY-MM-DD` | Empty stores `null` |
| `training_period` | string | No | Up to 100 characters | Trimmed; empty stores `null` |
| `speaker` | string | No | — | Trimmed; empty stores `null` |

`public_id`, timestamps, and deletion state are server-controlled.

**Status Code:** `201 Created`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `403 Forbidden`,
`500 Internal Server Error`

---

### 22. Update Training Catalog

Partially update an active catalog entry. Omitted fields keep their current
value; an empty string clears a nullable string field. `training_date` may be
changed but not cleared once set. A request that supplies no update fields is
rejected.

**Endpoint:** `PATCH /training-catalog/{publicID}`

**Authentication:** Admin.

**Request Fields:** The create fields as optional fields. `training_slots`, when
supplied, must be positive. At least one field is required.

**Response:** The updated catalog object.

**Status Code:** `200 OK`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `403 Forbidden`,
`404 Not Found`, `500 Internal Server Error`

---

### 23. Update Training Catalog Status

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

### 24. Soft Delete Training Catalog

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

### 25. Enroll in Training

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

The enrolling user, `register_date` (current date), and timestamps are
server-controlled. Enrollment is rejected with `409` when the catalog is
`completed`/unset, is full, or the user already has an active enrollment.
Enrollment is rejected with `404` when the catalog does not exist or is
soft deleted.

**Response:**

```json
{
  "public_id": "YTP-482920",
  "user_public_id": "YTP-000007",
  "register_date": "2026-09-17",
  "created_at": 1700000000,
  "updated_at": 1700000000,
  "catalog": {
    "public_id": "YTP-482910",
    "name": "Bisnis Digital",
    "training_status": "planned"
  }
}
```

**Status Code:** `201 Created`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `404 Not Found`,
`409 Conflict`, `500 Internal Server Error`

---

### 26. Cancel Training Enrollment

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

### 27. My Training Enrollment History

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
      "created_at": 1700000000,
      "updated_at": 1700000100,
      "catalog": {
        "public_id": "YTP-482910",
        "name": "Bisnis Digital",
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

### 28. All Training Enrollment History

List every user's enrollment history, including cancelled enrollments.

**Endpoint:** `GET /training-enrollments`

**Authentication:** Admin.

**Query Parameters:** `cursor` and `limit` as described for catalog listing.

**Response:** The enrollment page shown for `/training-enrollments/my`.

**Status Code:** `200 OK`

**Errors:** `400 Bad Request`, `401 Unauthorized`, `403 Forbidden`,
`500 Internal Server Error`

---

### 29. Catalog Training Enrollment History

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
- Public IDs use `YTP-DDDDDD`; they are lookup handles, not secrets or auth tokens.
- Password hashes are stored in `users.password` and never serialized.
- Refresh sessions store only SHA-256 token hashes plus expiry and revoke state.
- Enterprise turnover uses `DECIMAL(15,2)` and is serialized as a JSON string.
- Enterprise ownership is one-to-many: one user owns many enterprises, and
  `enterprises.user_id` is always server-derived from the authenticated user.
- Enterprise `name` and assessment enums are nullable; `business_sector` and
  `status` are required, and `status` defaults to `active`.
- Enterprise mutations write an `enterprise_audit_events` row (actor, action,
  changed fields JSONB, timestamp) in the same transaction. Migration `000003`
  adds the enterprise tables, the `business_sector_enum`, `enterprise_status_enum`,
  `legal_status_enum`, `business_digitization_enum`, `intervention_needs_enum`,
  `process_status_enum`, and `general_status_enum` types, and indexes on
  `user_id`, `district`, `status`, `business_sector`, `legal_status`,
  `business_digitization`, `intervention_needs`, `training_status`,
  `mentoring_status`, `capital_access`, `partnership`, `deleted_at`, plus
  cursor-friendly and composite owner/deleted/id combinations.
- Training catalog optional fields are nullable; `training_slots` is a positive
  `INTEGER` or `NULL` for unlimited. Catalog `training_status` reuses
  `process_status_enum`; enrollment creation reserves capacity while the status
  is `planned` or `ongoing`.
- Enrollment `register_date`, `training_date`, and `training_period` are stored
  as written; `register_date` is server-derived from the current date.
- Enrollment cancellation is a soft delete that preserves history, and the
  active uniqueness index allows re-enrollment after cancellation.
- Migration `000005` adds `training_catalog` and `training_enrollments` with a
  positive `training_slots` check, a partial unique active-enrollment index, and
  indexes on catalog `deleted_at`, `training_date`, `category`, `training_status`,
  and cursor, plus enrollment `training_catalog_id`, `user_id`, `deleted_at`,
  active-catalog, and active-user-cursor combinations. It reuses the existing
  `process_status_enum` and does not recreate it.

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
