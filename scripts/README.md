# Samba Domain-Member Scripts

This directory contains the minimal scripts needed to join a long-running
container (or k8s pod) to an Active Directory domain so the wbclient-go HTTP
service can authenticate users via `libwbclient` → `winbindd`.

## Files

| File | Purpose |
|---|---|
| `domain-join.sh` | One-shot join script. Run inside the container via `docker exec` (or as a k8s init container). Writes `krb5.conf` + `smb.conf`, runs `net ads join`, starts `winbindd`. |
| `domain-leave.sh` | One-shot leave script. Stops `winbindd`, runs `net ads leave` to remove the AD computer account, clears persisted `secrets.tdb` / `smb.conf` / `krb5.conf` so the pod ends in a clean unjoined state. |

## What was removed from `docker/setup-samba-member.sh`

The original 17-step script provisioned both a Samba member join **and**
SSSD/PAM/NSS integration so AD users could log in interactively, run shells,
etc. Our service does none of that — it only needs `winbindd` running with a
valid machine credential. Everything else was dead weight.

### Steps removed from the script flow

| Step removed | Reason it's not needed |
|---|---|
| Edit `/etc/hosts` | DNS is pinned to the DC at container creation time via Docker's `dns:` directive. |
| Rewrite `/etc/resolv.conf` | Same reason — resolver is already correct at boot. |
| Start D-Bus | Only needed by `realmd`/SSSD; we run neither. |
| `realm discover` | Diagnostic for `realmd`; `net ads join` doesn't need it. |
| Configure SSSD | SSSD provides PAM/NSS for OS-level AD logins. We don't do OS-level logins. |
| Start SSSD | Same. |
| Update `nsswitch.conf` (`sss` entries) | NSS-only; `libwbclient` talks to `winbindd` over its UNIX socket and bypasses NSS entirely. |
| Admin `kinit` | One-shot bootstrap ticket that nothing later consumes. `net ads join -U user%pass` does its own auth. |
| Verification block (`wbinfo -u`, etc.) | Diagnostic; not part of joining. |

### `smb.conf` lines removed

| Line removed | Reason |
|---|---|
| `winbind enum users` / `winbind enum groups` | NSS-only — would let `getent passwd` enumerate AD users. |
| `template shell` / `template homedir` | Defaults for OS-level shell logins; we never do them. |
| `password server` | DNS SRV records (`_kerberos._tcp`, `_ldap._tcp`) locate the DC; explicit pin not needed when DNS is healthy. |
| `name resolve order` | Only matters when DNS is broken. We assume it isn't. |
| `dns proxy = no` | Only relevant when running `nmbd`, which we don't. |

### `krb5.conf` entries removed

The original had three `[realms]` blocks (one each for `${REALM}`,
`${DOMAIN_NAME}`, `${WORKGROUP}`) — duplicate aliases that no caller uses.
The minimal `krb5.conf` keeps only one realm block.

End result: 7 active steps instead of 17, a much smaller `smb.conf`, no
SSSD/PAM/NSS state to maintain.

## Flow of `domain-join.sh`

1. **Input validation** — fail fast if `DC_FQDN`, `NETBIOS_NAME`,
   `AD_USERNAME` or `AD_PASSWORD` is missing.
2. **Realm discovery** — `net ads info -S ${DC_FQDN}` (pure CLDAP, no
   credentials needed) returns the realm authoritatively. **No fallback** —
   if the query fails or returns no `Realm:` field the script exits with a
   non-zero status. The DC is the only authoritative source for the realm
   name; guessing from the DC FQDN can produce a wrong realm that would
   then cause the join to fail in much more confusing ways. The only way
   to bypass the query is to set `REALM` explicitly in the environment
   before invoking the script. `DOMAIN_NAME` defaults to lowercase of
   `REALM`.
3. **Ensure directories / back up configs** — `mkdir -p /var/log/samba`;
   move existing `smb.conf` / `krb5.conf` to `.original` (idempotent).
4. **Write minimal `/etc/krb5.conf`** — `default_realm`, one `[realms]`
   block pointing at `DC_FQDN` as the KDC, and `[domain_realm]` mappings.
5. **Write minimal `/etc/samba/smb.conf`** via `crudini`.
6. **`net ads join -S ${DC_FQDN} -U user%pass`** + `net ads testjoin`. The
   `-S` pins the join to the specific DC instead of letting Samba pick a
   DC via discovery — important because the realm we discovered came from
   that same DC, so we want to stay consistent.
7. **Start `winbindd -D`** — the daemon `libwbclient` talks to.
8. **`wbinfo -t`** — live Kerberos handshake against the DC to confirm the
   trust is healthy before declaring success.

## Flow of `domain-leave.sh`

The mirror image of `domain-join.sh` — undoes a successful join so the
pod is back to an unjoined state.

1. **Input validation** — fail fast if `AD_USERNAME` or `AD_PASSWORD` is
   missing. Admin credentials are required because `net ads leave`
   deletes the AD-side computer account, which the machine account
   itself typically doesn't have permission to do.
2. **Stop `winbindd`** — once the trust is about to be broken, the
   daemon shouldn't be holding tickets against a soon-stale identity.
3. **`net ads leave -U user%pass`** — authenticates to AD with the admin
   credentials, deletes the computer account, and clears Samba's
   internal trust state in `secrets.tdb`.
4. **Wipe persisted state** — explicitly remove `secrets.tdb`,
   `smb.conf` and `krb5.conf`. After this, the volume contents look
   like a freshly-provisioned pod that has never been joined.

## `smb.conf` flag reference

| Flag | Value we set | Why it's set | Required? |
|---|---|---|---|
| `security` | `ads` | Selects AD-member mode (vs. standalone/NT4). | **Mandatory** |
| `realm` | `${REALM}` | Kerberos realm `winbindd` uses for AS-REQ / TGS-REQ. | **Mandatory** |
| `workgroup` | `${NETBIOS_NAME}` | NetBIOS short domain name. Samba refuses to start in ADS mode without it. | **Mandatory** |
| `ntlm auth` | `yes` | Enables NTLM-family auth path. The service's MSCHAPv2 flow (`wbcAuthenticateUserEx` with NT-Response) needs this. Modern Samba defaults disable plain NTLM for security; we re-enable explicitly. | **Mandatory** |
| `client ntlmv2 auth` | `yes` | Forces NTLMv2 (not the legacy NTLMv1) when Samba initiates NTLM. Belt-and-suspenders alongside `ntlm auth`. | **Mandatory** |
| `winbind use default domain` | `yes` | Lets callers pass `alice` instead of `CORP\alice` to `wbinfo`/`wbcAuthenticateUserEx`. Improves caller ergonomics. | Nice-to-have |
| `machine password timeout` | `${MACHINE_PASSWORD_TIMEOUT:-2592000}` (script reads the env var; default 2592000s = 30 days) | How often `winbindd` rotates the long-term machine password against AD. The value is supplied by the caller through the `passwordTimeout` field of `DomainJoinReq` (in days). If the request omits it, `apiDomainJoin` defaults to 30 days; if the script is invoked directly without the env var, the same 30-day default applies. | Nice-to-have / caller-tunable |

## `krb5.conf` flag reference

### `[libdefaults]`

| Flag | Value | Why | Required? |
|---|---|---|---|
| `default_realm` | `${REALM}` | Realm assumed when a principal is named without one. MIT Kerberos can't function without this. | **Mandatory** |
| `dns_lookup_realm` | `false` | We've set `default_realm`; avoid an unnecessary DNS round trip. | Nice-to-have |
| `dns_lookup_kdc` | `false` | We list the KDC explicitly below; don't depend on `_kerberos._tcp` SRV records being correctly published. | Nice-to-have |
| `udp_preference_limit` | `0` | Forces all Kerberos requests over TCP. Default tries UDP first and falls back; AD's PAC structures are large enough to fragment on UDP and trigger silent retries. | Nice-to-have |

### `[realms] ${REALM} = { ... }`

| Flag | Value | Why | Required? |
|---|---|---|---|
| `kdc` | `${DC_FQDN}` | Where to send AS-REQ / TGS-REQ. With `dns_lookup_kdc = false`, this is mandatory. | **Mandatory** |
| `admin_server` | `${DC_FQDN}` | Where `kadmin` / `kpasswd` go. Rarely used by us but conventionally required. | **Mandatory** |
| `default_domain` | `${REALM}` | Default domain used when constructing principals from bare usernames. | Nice-to-have |

### `[domain_realm]`

| Flag | Value | Why | Required? |
|---|---|---|---|
| `.${DOMAIN_NAME}` | `${REALM}` | Maps any subdomain of the AD zone → the realm. Used when MIT Kerberos sees a hostname and needs to figure out which realm it belongs to. | **Mandatory** |
| `${DOMAIN_NAME}` | `${REALM}` | Same mapping for the bare zone (no host part). | **Mandatory** |

## TGTs in our auth path

There are three Kerberos credential concepts in play. Our HTTP service uses
only one of them — the rest exist either as transparent internals of
`winbindd` or as artefacts of other auth paths we don't use.

| Credential | Where it lives | Role | Used by our service? |
|---|---|---|---|
| User TGT | Cred cache populated by `pam_winbind` during a user login | Lets the logged-in user use Kerberos-protected services on the box | **No** — we never do PAM logins |
| Machine TGT | In-memory `MEMORY:` cred cache inside `winbindd` | Used to authenticate `winbindd` to the DC for every NetLogon RPC | **Yes** — transparently; we never touch it |
| Machine password | Long-term symmetric key in `/var/lib/samba/private/secrets.tdb` | The load-bearing credential `winbindd` derives the machine TGT from | **Yes** — this is what matters |

Every HTTP authentication request the service receives becomes
`wbcAuthenticateUserEx` → `winbindd`'s `NetrSamLogonEx` RPC → DC. The
machine TGT is already cached inside `winbindd` (acquired once at startup
from the machine password); the per-request work is just the RPC carrying
the user's NT-Response.

**Why we never `kinit`:** `kinit` creates a user TGT in a `FILE:` cred
cache for the current user (root, in the container). `winbindd` doesn't
read that cache — it manages its own `MEMORY:` cache from the machine
password directly. Running `kinit` would create a credential nothing
consumes.

**Why `klist` shows nothing:** `klist` reads the current user's `FILE:`
cred cache (empty — nobody ran `kinit`), or `/etc/krb5.keytab` (not
written by `net ads join` with the default `kerberos method = secrets
only`). The credentials are present, just in `secrets.tdb` and inside
`winbindd`'s address space — neither of which `klist` can read. Use
`wbinfo -t` instead; it performs a live handshake against the DC and is
the canonical "is my trust healthy?" check.

## Test cases that shaped the script

| # | What we tried | Result | What it taught us |
|---|---|---|---|
| 1 | Pass the AD domain FQDN (e.g. `corp.example.com`) as the DC argument | `net ads info` couldn't fetch the realm — the command needs a specific server, not the bare domain. The script now exits with a clear error rather than guessing | The input must be a **DC FQDN** (e.g. `dc.corp.example.com`), not the AD zone |
| 2 | Pass the DC FQDN (e.g. `dc.corp.example.com`) | `net ads info -S <fqdn>` returned the realm authoritatively; the join succeeded; `wbinfo -t` verified | This is the right shape — discover the realm from a specific DC, then `net ads join -S <fqdn>` to pin the join to that same DC for consistency |
| 3 | Drive a short-interval rotation by setting `MACHINE_PASSWORD_TIMEOUT=60` when invoking `domain-join.sh` (or by setting `machine password timeout = 60` in `smb.conf` directly), restart the container, wait > 90 s, run `wbinfo --authenticate=user%pass` | Auth still succeeded; `net ads info` reported an updated `Last machine account password change` timestamp | Confirms the machine-password rotation works end-to-end — `winbindd` rotates against the DC and updates `secrets.tdb` atomically, with no operator action |

## Persistence requirements for container/pod restarts

Two pieces of state **must** survive container restart for `winbindd` to
come back up without re-joining:

| Path | What's in it | Compose volume |
|---|---|---|
| `/var/lib/samba/private/` | `secrets.tdb` (the machine credential) plus other Samba TDBs | `samba-state-netads` |
| `/etc/samba/` | `smb.conf` (so `winbindd` knows the realm/workgroup) | `samba-config-netads` |

Additionally the **container hostname must be stable** across restarts —
the AD computer account is named after the hostname (`WBCLIENT-TEST$` for
hostname `wbclient-test`), and the persisted `secrets.tdb` is bound to
that account name. In the compose file we set `hostname: wbclient-test`
explicitly to prevent Docker from assigning a new container-ID-based
hostname on recreate. In Kubernetes, use a `StatefulSet` (pod name →
stable hostname) or a single-replica `Deployment` with an explicit
`hostname:` field.

## Pod requirements for production deployment

The wbclient-go service exposes two domain-lifecycle endpoints:

- **`POST /samba.domain.join`** → `apiDomainJoin` → `scripts/domain-join.sh`
- **`POST /samba.domain.leave`** → `apiDomainLeave` → `scripts/domain-leave.sh`

A typical production workflow:

1. Pod boots with the wbclient-go service running.
2. Operator (or automation) calls `POST /samba.domain.join` with a JSON
   body matching `DomainJoinReq`:
   ```json
   {
     "dcfqdn":          "dc.corp.example.com",
     "netbiosName":     "CORP",
     "adUsername":      "joiner@corp.example.com",
     "adPassword":      "...",
     "passwordTimeout": 30
   }
   ```
   The handler translates these to the env vars `domain-join.sh` expects
   (`DC_FQDN`, `NETBIOS_NAME`, `AD_USERNAME`, `AD_PASSWORD`,
   `MACHINE_PASSWORD_TIMEOUT`), runs the script, and returns
   `DomainOpsResp`.

   **`passwordTimeout`** is optional. It is interpreted in **days** and
   the handler converts it to seconds before passing it to the script as
   `MACHINE_PASSWORD_TIMEOUT` — which becomes the
   `machine password timeout` setting in `smb.conf` (how often
   `winbindd` rotates the long-term machine password against AD). If the
   field is omitted or non-positive, the handler defaults to **30 days**.
3. If the pod is already joined and `net ads testjoin` passes, the
   handler short-circuits with `{success: true, errorMessage: "already
   joined"}` — safe to call repeatedly, idempotent for healthy state.
4. Once the script completes, `winbindd` is running and the
   `/samba.user.validate` / `/samba.user.auth` endpoints can authenticate
   users immediately.
5. To decommission, the operator calls `POST /samba.domain.leave` with a
   JSON body matching `DomainLeaveReq`:
   ```json
   {
     "domain":     "dc.corp.example.com",
     "adUsername": "joiner@corp.example.com",
     "adPassword": "..."
   }
   ```
   The handler short-circuits if the pod isn't currently joined
   (`{success: true, errorMessage: "not joined"}`), otherwise runs
   `domain-leave.sh`. After a successful leave the AD computer account
   is gone and the persisted Samba state is wiped, so the pod ends in
   the same state as a freshly-provisioned one.

Both endpoints can be made configurable via env vars:
`WBCLIENT_DOMAIN_JOIN_SCRIPT` and `WBCLIENT_DOMAIN_LEAVE_SCRIPT` override
the default paths (`/usr/src/scripts/domain-{join,leave}.sh`).

### What the pod itself must provide

| Requirement | Why | How |
|---|---|---|
| Persistent volumes on `/var/lib/samba/private/` and `/etc/samba/` | So a pod restart doesn't lose `secrets.tdb` / `smb.conf` and force a rejoin | PVCs in k8s (`volumeClaimTemplates` in a StatefulSet, or PVCs bound to a single-replica Deployment) |
| Stable hostname across restarts | AD computer account name is derived from hostname; the persisted `secrets.tdb` is tied to that account | StatefulSet (pod name → hostname) or `spec.hostname:` on a single-replica Deployment |
| `scripts/domain-join.sh` and `scripts/domain-leave.sh` accessible inside the pod | The handlers shell out to them | Bake into the image, or override paths via env vars `WBCLIENT_DOMAIN_JOIN_SCRIPT` / `WBCLIENT_DOMAIN_LEAVE_SCRIPT` |
| `bash`, `samba`, `winbind`, `crudini` installed | Script needs them | Image dependency |
| Privileged enough to write `/etc/krb5.conf`, `/etc/samba/smb.conf` and run `winbindd` | These live in protected paths | Run the container as root, or grant the necessary file capabilities |
| DNS that resolves the DC FQDN | Used by `net ads info` (realm discovery) and the join itself | Either pin the resolver to the AD DNS server (k8s `dnsConfig.nameservers` / Docker `--dns`) or rely on the cluster DNS being able to find the DC zone |

### Pod startup logic

The wbclient-go binary's `Start()` does a best-effort `winbindd -D` at
startup (see `restoreWinbindd()` in `service.go`). Behaviour by scenario:

- **Fresh pod** (first ever boot, no persisted state): `winbindd -D` fails
  harmlessly because there's no `smb.conf` yet — the error is logged and
  the HTTP server still comes up. `/samba.user.*` endpoints return errors
  until the operator calls `POST /samba.domain.join`, which runs
  `domain-join.sh` and starts `winbindd`.
- **Pod restart after a successful join**: the PVCs have `secrets.tdb`
  and `smb.conf`. `winbindd -D` starts cleanly and auth APIs work
  immediately — no operator action required.

No separate entrypoint script is needed. The production container's CMD
is just the wbclient-go binary.

`winbindd` is started **best-effort, not supervised** — if it dies later
during pod lifetime, the wbclient-go service does not attempt to restart
it. Operators are expected to detect this through their own external
health monitoring and recreate the pod (which goes through the same
startup path again).
