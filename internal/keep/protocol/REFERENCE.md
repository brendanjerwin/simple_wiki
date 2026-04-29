# Reverse-engineered Google Keep protocol — pinned reference

This package is a Go port of the gpsoauth + Google Keep wire protocol. There is no
public Google Keep API. We mirror the Python `gkeepapi` library, which is the
de-facto community reference, at a specific pinned commit so future readers can
diff against the upstream when something breaks.

> **Warning.** This protocol is unofficial. Google can change it without notice
> and has done so several times in the past. When pulls/pushes start returning
> `protocol_drift` errors, the diagnostic flow is: open the upstreams below at
> their current `main`, diff against the pinned commits, and apply the deltas.

## Pinned upstreams

| Upstream | Repo | Pinned commit |
| --- | --- | --- |
| `gkeepapi` (Keep client + node model) | <https://github.com/kiwiz/gkeepapi> | `1a94b25c18a7abfdc23d1412091129cd63652877` |
| `gpsoauth` (Android-style auth) | <https://github.com/simon-weber/gpsoauth> | `429b7f99fa268315cef7a981408a612fb424a79b` |

The mapping below is what we mirror in Go. Anything not listed is intentionally
not implemented (see "Out of scope").

## Auth flow

Two-stage exchange that mimics what the Android Keep app does on first launch:

```text
                +----------------------+
   ASP -------->|  master_login        |--> master token (oauth2rt_1/...)
                |  (or exchange_token) |
                +----------------------+
                            |
                            v
                +----------------------+
   master ----->|  oauth (per-service) |--> short-lived bearer (~1h)
                +----------------------+
                            |
                            v
                +----------------------+
                |  Keep API            |
                |  Authorization:      |
                |  OAuth <bearer>      |
                +----------------------+
```

### Stage 1 — Master login (we use App-Specific Password)

- **URL**: `POST https://android.clients.google.com/auth`
- **Content-type**: `application/x-www-form-urlencoded`
- **User-Agent**: `GoogleAuth/1.4`  (Google rejects requests without this exact UA)
- **Form body** (gpsoauth `perform_master_login` for ASPs; `exchange_token` for
  browser-flow `oauth_token` cookies):

  | field | value |
  | --- | --- |
  | `accountType` | `HOSTED_OR_GOOGLE` |
  | `Email` | the user's Google email |
  | `has_permission` | `1` |
  | `add_account` | `1` |
  | `EncryptedPasswd` | RSA-OAEP-encrypted `email\x00password` (see below) |
  | `service` | `ac2dm` |
  | `source` | `android` |
  | `androidId` | a stable 16-hex-char device id (we generate one per profile and persist it) |
  | `device_country` | `us` |
  | `operatorCountry` | `us` |
  | `lang` | `en` |
  | `sdk_version` | `17` |
  | `google_play_services_version` | `240913000` |
  | `client_sig` | `38918a453d07199354f8b19af05ec6562ced5788` |
  | `callerSig` | `38918a453d07199354f8b19af05ec6562ced5788` |
  | `droidguard_results` | `dummy123` |

- **Response**: `text/plain` key=value lines. Success iff body contains a `Token=`
  line (the master token, prefixed `oauth2rt_1/`). Other useful keys: `Email`,
  `Error`, `ErrorDetail`, `Url` (`Url` set when `Error=NeedsBrowser` —
  treat as `ErrAuthRevoked`/needs-rebind).

### Stage 2 — Per-service OAuth bearer (Keep)

- **URL**: same — `POST https://android.clients.google.com/auth`
- **Form body** (gpsoauth `perform_oauth`):

  | field | value |
  | --- | --- |
  | `accountType` | `HOSTED_OR_GOOGLE` |
  | `Email` | the user's Google email |
  | `has_permission` | `1` |
  | `EncryptedPasswd` | the **master token** verbatim (yes, password slot is reused) |
  | `service` | `oauth2:https://www.googleapis.com/auth/memento` |
  | `source` | `android` |
  | `androidId` | same id as stage 1 |
  | `app` | `com.google.android.keep` |
  | `client_sig` | `38918a453d07199354f8b19af05ec6562ced5788` |
  | `device_country`, `operatorCountry`, `lang`, `sdk_version`, `google_play_services_version` | same as stage 1 |

- **Response**: `Auth=<bearer>`. The bearer is short-lived (≈1h); refresh on any
  401 from Stage 3 by re-running Stage 2.

### Stage 3 — Keep API

- **Base URL**: `https://www.googleapis.com/notes/v1/`
- **Auth header**: `Authorization: OAuth <bearer>` (mirrors gkeepapi `_send` line 271; `GoogleLogin auth=…` is the legacy Android format and now returns 401 from googleapis.com)
- **Sync endpoint**: `POST /notes/v1/changes`
- **Request body** (JSON):

  ```json
  {
    "nodes": [],
    "clientTimestamp": "<microseconds-since-epoch as decimal string>",
    "requestHeader": {
      "clientSessionId": "s--<ms>--<10digits>",
      "clientPlatform": "ANDROID",
      "clientVersion": {"major": "9", "minor": "9", "build": "9", "revision": "9"},
      "capabilities": [
        {"type": "NC"}, {"type": "PI"}, {"type": "LB"}, {"type": "AN"},
        {"type": "SH"}, {"type": "DR"}, {"type": "TR"}, {"type": "IN"},
        {"type": "SNB"}, {"type": "MI"}, {"type": "CO"}
      ]
    },
    "targetVersion": "<cursor>",   // optional; omit for full pull
    "userInfo": {"labels": [...]}  // optional
  }
  ```

  - The `nodes` array carries inbound mutations: each entry is a Note (`type=NOTE`
    or `type=LIST`) or ListItem (`type=LIST_ITEM`) with the standard Keep node
    fields (`id`, `serverId`, `parentId`, `type`, `text`, `checked`, `sortValue`,
    `timestamps`, `kind`, etc. — see `gkeepapi/src/gkeepapi/node.py` for the full
    surface).
  - `clientTimestamp` is microseconds (gkeepapi's `NodeTimestamps.int_to_str`
    multiplies seconds by 1e6).

- **Response body** (JSON):

  ```json
  {
    "toVersion": "<new cursor>",
    "nodes": [...],   // changed nodes (inbound diff)
    "userInfo": {"labels": [...]},
    "forceFullResync": <bool>,   // when true: discard cursor, do full reconcile
    "truncated": <bool>          // when true: more data available, call again with new cursor
  }
  ```

### Capability flags (request)

We send the same capability set gkeepapi sends. They affect what fields the
server is willing to round-trip:

- `NC` color, `PI` pinned, `LB` labels, `AN` annotations, `SH` sharing,
- `DR` drawing, `TR` trash (no longer set deletion timestamp), `IN` indentation,
- `SNB` shared-note modification, `MI` concise blob info, `CO` VSS support.

We do **not** advertise `EC`/`RB`/`EX` (gkeepapi marks them as unknown).

### Node identity

- `id`: client-side ULID-style identifier we control. We keep our wiki UID per
  item and store the Keep `id` per-binding under `agent.connectors.google_keep.bindings[<note_id>].id_map[<wiki_uid>]`.
- `serverId`: Keep-assigned id once a node is sync'd. After first round-trip,
  this is the stable handle.
- `parentId`: for ListItems, the containing Note's id (or `serverId` after first
  sync).

## Encrypted password construction

`EncryptedPasswd` is what the gpsoauth library calls a "signature":

1. Decode `B64_KEY_7_3_29` (the Android Play Services 7.3.29 RSA public key,
   base64'd in the gpsoauth source).
2. Parse it as a packed `(modulus, exponent)` blob:
   - First 4 bytes BE = `i` = modulus length.
   - Next `i` bytes = modulus (big-endian unsigned int).
   - Next 4 bytes BE = `j` = exponent length.
   - Next `j` bytes = exponent.
3. Build a key-struct: `\x00\x00\x00\x80` + modulus_bytes + `\x00\x00\x00\x03` +
   exponent_bytes.
4. SHA-1 the key-struct; take the first 4 bytes as the "key fingerprint."
5. RSA-OAEP encrypt `email + "\x00" + password_or_master_token` with the key.
   - Hash and MGF1 default to SHA-1 (PyCryptodome `PKCS1_OAEP.new(key)` defaults).
6. Concatenate: `\x00` (version byte) + 4-byte fingerprint + RSA-OAEP ciphertext.
7. Base64 URL-safe encode (no padding handling — Python's `urlsafe_b64encode`
   keeps `=` padding; preserve it).

The fixed RSA key blob (from gpsoauth):

```text
AAAAgMom/1a/v0lblO2Ubrt60J2gcuXSljGFQXgcyZWveWLEwo6prwgi3iJIZdodyhKZQrNWp5nKJ3srRXcUW+F1BD3baEVGcmEgqaLZUNBjm057pKRI16kB0YppeGx5qIQ5QjKzsR8ETQbKLNWgRY0QRNVz34kMJR3P/LgHax/6rmf5AAAAAwEAAQ==
```

## Out of scope (what we are explicitly not porting)

- Media/blob upload (`https://keep.google.com/media/v2/`) — no image attachments
  in v1.
- Labels API mutation — we don't tag Keep items via Keep labels; tags ride in
  item text as `#tag` markers (consistent with the CalDAV bridge, #983).
- Drawings, annotations beyond what comes back in a full node (we read but
  don't write).
- Family/sharing endpoints.

## Failure surface (mapped to typed errors in `errors.go`)

| Upstream signal | Our error | Typical cause |
| --- | --- | --- |
| `Error=BadAuthentication` (Stage 1) | `ErrInvalidCredentials` | Wrong ASP. |
| `Error=NeedsBrowser` (Stage 1) | `ErrAuthRevoked` | Account requires browser interaction; user must regenerate ASP. |
| `Error=…` other (Stage 1) | `ErrAuthRevoked` | 2SV not enabled, account suspended, etc. |
| HTTP 401 (Stage 3, after refresh) | `ErrAuthRevoked` | Master token revoked. |
| HTTP 429 (Stage 3) | `ErrRateLimited` | Backoff per Phase D. |
| HTTP 404 (`note not found` shape on a bound note) | `ErrBoundNoteDeleted` | The user deleted the bound Keep note. |
| Decoder rejects shape | `ErrProtocolDrift` | Google changed the wire format. |

## Stability notes

- `client_sig`/`callerSig` value is the Google Play Services 7.3.29 cert hash.
  Older gkeepapi versions used different sigs that Google has since revoked.
- `google_play_services_version` is bumped occasionally by upstream gpsoauth.
  When we see auth failures on accounts that previously worked, this is the
  first knob to bump (mirror upstream).
- `User-Agent: GoogleAuth/1.4` is **required**. Different UAs return 403.
- TLS client hints matter — gpsoauth disables ALPN and OP_NO_TICKET. The Go
  port should use `tls.Config{NextProtos: nil}` and `SessionTicketsDisabled: false`
  to mimic. (Specifically: do *not* advertise h2/http2 ALPN.)

## Field-level quirks (the painful list)

Every row here was discovered by hitting the symptom on a real account, A/B-testing
via `cmd/keep-debug`, and tracing back to the gkeepapi behavior or comparing wire
bodies. Treat this as the canonical reference when something starts 500-ing again.

### Auth

| Quirk | Symptom on miss | Source |
| --- | --- | --- |
| App-Specific Passwords are deprecated for most accounts; only the `oauth_token` cookie capture from `accounts.google.com/EmbeddedSetup` works | Stage 1 returns `BadAuthentication` regardless of ASP correctness | gkeepapi alt-flow doc; observed real-account |
| The auth HTTP client must NOT advertise `h2` in TLS ALPN | Stage 1 returns 403 before body parsing | gpsoauth Python source |
| `Authorization` header for Keep API is `OAuth <bearer>`, NOT `GoogleLogin auth=<bearer>` | Stage 3 returns 401 "Login Required" | gkeepapi `__init__.py:271` |
| Master token format may be `aas_et/...` (not just `oauth2rt_1/...`) | n/a — accept both | observed real-account |
| User must click "I agree" in the EmbeddedSetup flow even if the page hangs on a spinner — the cookie is set after consent regardless | User reports "I copied the cookie but it doesn't work" | gpsoauth alt-flow procedure |

### Timestamps

| Quirk | Symptom on miss | Source |
| --- | --- | --- |
| `clientTimestamp` is RFC3339 microseconds (`2006-01-02T15:04:05.000000Z`), NOT a microseconds integer | Stage 3 returns 400 "Invalid format … is malformed" | gkeepapi `node.py:int_to_str` |
| Exact `1970-01-01T00:00:00.000Z` is the "no timestamp" sentinel — treat as zero | Otherwise alive items decode as trashed in 1970 | observed real-account |
| Epoch + millisecond offsets (`1970-01-01T00:00:00.001Z`/`.002Z`) are REAL placeholder timestamps Keep stamps on items it created server-side; do NOT treat as zero | Push gates always think wiki is newer; pushes happen forever | observed real-account, today |
| `userEdited` carries the actual "user touched this" recency on items that have been toggled via the phone app; `updated` can stay at the millisecond-epoch sentinel forever | Inbound apply skips phone-side toggles → wiki never reflects checks | observed real-account, today |
| `updated` on an UPDATE push must NOT be older than the server's record | Stage 3 returns 500 "Unknown Error" | observed real-account |
| Don't emit `created` on UPDATE pushes — only on creates | Stage 3 returns 500 "Unknown Error" | observed real-account; matches gkeepapi pattern |

### Node identity & shape

| Quirk | Symptom on miss | Source |
| --- | --- | --- |
| Node has BOTH `id` (client-generated stable identifier from item creation) AND `serverId` (server-assigned). They are DIFFERENT values; sending `id == serverId` rejects | Stage 3 returns 500 | gkeepapi `Node.save()` |
| LIST_ITEM updates require `parentServerId` AS WELL AS `parentId` | Stage 3 returns 500 | gkeepapi `node.py:1585` |
| `checked` field must always be emitted (no `omitempty`); a missing `checked` field is interpreted as "set to false" by Keep, not "leave alone" | Phone-side checks revert on next outbound push | observed real-account, today |
| Items have `kind: "notes#node"` | Stage 3 returns 400 if missing | gkeepapi `Node.save()` |
| `IN` capability flag in `requestHeader.capabilities` is required to push LIST_ITEMs with `parentId` | Stage 3 returns 500 | gkeepapi `__init__.py:346` |

### Wire-protocol oddities

| Quirk | Symptom on miss | Source |
| --- | --- | --- |
| Creating a LIST and pushing its initial children must happen in ONE `Changes` request — items reference the LIST's CLIENT id (server hasn't issued a server id yet) | Two-step (create list, then push items) returns 500 | observed real-account, today |
| Push must include `targetVersion` from a fresh pull on every incremental update | Stage 3 returns 500 on update-style pushes | gkeepapi sync flow |
| **Multi-item update where every item is byte-identical to the server's current state → 500 "Unknown Error"**. Keep rejects empty-diff pushes | Cron-tick pushes 500 forever | observed real-account, today; matches gkeepapi's `_findDirtyNodes` pattern |

### Response handling

| Quirk | Action required | Source |
| --- | --- | --- |
| `forceFullResync: true` | Drop local cursor, rebuild from a no-targetVersion pull | gkeepapi `__init__.py:1117` (#70) |
| `upgradeRecommended: true` | Surface as a warning; we have no real upgrade path on a reverse-engineered client | gkeepapi `__init__.py:1120` (#70) |
| `truncated: true` | Loop the pull with the new `toVersion` until `truncated: false` | gkeepapi `__init__.py:1134` (#70) |
| Response body for non-empty arrays of nested maps strips the parent key from the index — implication: never query `wiki.connectors.google_keep.bindings` via the wiki frontmatter index, query a leaf string field like `email` instead | Bindings invisible to cron lister at startup | wiki frontmatter index `indexArray` |

### Diagnostic flow when something 500s

1. Run `cmd/keep-debug -cmd=update-item -parent-id=<list-server-id> -item-text=<some-item-text>` against a fresh sandbox list to confirm the wire-shape is sane.
2. If sandbox succeeds and a real list fails: it's data-state-specific — usually some item-level corruption from a prior broken sync or a no-op-update rejection.
3. Use `cmd/keep-debug -cmd=raw-pull -parent-id=<list-server-id>` to dump the literal JSON Keep returns; compare against gkeepapi's expected save() output.
4. The wiki's Changes() debug-logger prints the request body verbatim (with bearer/master-token redacted) and the response body. Set `truncateBody`'s `maxLen` higher than the response size temporarily if you need to see past the userInfo block.
