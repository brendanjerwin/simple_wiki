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
                |  GoogleLogin auth=…  |
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
- **Auth header**: `Authorization: GoogleLogin auth=<bearer>`
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
