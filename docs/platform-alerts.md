# Platform alerts

High-level investigation for Capp platform alerts: common causes, how to find the root cause, and how to resolve it.

---

## Backend and frontend availability

The management plane is down or erroring: the **web console** or the **API**. Users cannot sign in, create, or manage Capps. Running Capps can still serve traffic.

### Common causes

- **Console or API workload** — instances not running or not healthy.
- **Sign-in** — the identity provider is down or rejecting login (console and API both fail for users).
- **Cluster access** — the API instances are up but cannot talk to the cluster, so manage operations fail.

### How to investigate

1. What fails: **console**, **API**, or **both**?
2. Are that workload's instances running and healthy? If **not**: its logs. If **yes**: continue.
3. If instances **are** healthy:
   - **One user** — account, credentials, console URL, or browser.
   - **Everyone**, console fails, API or CLI works: console URL or the path to the UI.
   - **Everyone**, fail at login: identity provider (backend logs if login reaches the API).
   - **Everyone**, login works, API errors: the API cannot reach the cluster — backend logs.

### How to resolve

- **Sign-in, one user** — Wrong account or credentials: correct them.
- **Console only, API or CLI works** — Wrong console URL, or a client/browser issue.

---

## High operator reconcile rate

The operator is reconciling too often. That loads the control plane. Capps may still look Ready.

### Common causes

- **Spec flapping** — this Capp is updated again and again, so the operator never rests.
- **Same Capp retried** — spec is unchanged, but the same Capp is retried every loop (child never stable, or the same error).
- **Many Capps** — a large number of Capps, or a shared system flapping, so almost every Capp requeues.

### How to investigate

1. Operator logs: one Capp or many? Which Capp appears most?
2. For that Capp: spec changing over and over, or spec unchanged and the same error / child oscillation every loop?
3. If **many** Capps: shared cause (operator, API, or a shared child system).

### How to resolve

- **Spec flapping** — Something is rewriting this Capp: stop those updates or fix the writer.
- **Same Capp retried** — Spec is not changing, but the same Capp is retried every loop: operator controller bug.
- **Many Capps** — The operator is overloaded, or a shared system is forcing retries.

---

## Capp readiness time

Capps are becoming usable too slowly after create or update, or never become Ready. Ready means the required child resources are provisioned and healthy.

### Common causes

- **Image pull** — large image, slow or unreachable registry, missing registry credentials, or rate limits.
- **Scheduling** — the new workload cannot run: the Capp namespace quota is at its limit, or the cluster has no capacity.
- **Slow or failing startup** — the container starts late, crashes, or readiness probes never succeed.
- **Waiting on a child** — a required child can hold Ready after the workload is up.
- **Operator cannot sync** — a child was never created or updated.
- **Operator or shared platform is behind** — many Capps slow at once, rather than one workload.

### How to investigate

1. Which Capps are slow or not Ready? One Capp or many?
2. On that Capp, read the Ready **reason** and **message**.
3. Look at that Capp's child resources: do they exist, and are they healthy? Missing or stuck: operator logs — is the operator creating children?
4. If the reason is the **app**: check image pull, scheduling, start, and probes. If it cannot schedule, check whether the namespace ResourceQuota is at its limit.
5. If the reason is a **child**: that child's status.
6. If **many** Capps share the same reason: one shared cause (operator or that child).

### How to resolve

- **Image pull** — Wrong image or pull credentials: correct them. Registry unreachable: the registry is down or blocking the pull.
- **Scheduling** — Namespace over quota: reduce what this Capp requests so it fits. Quota not exhausted and the workload still cannot run: the cluster is out of capacity.
- **Startup / probes** — Crashing: fix the application. Up but not Ready: fix the app or the readiness check.
- **Operator cannot sync** — Invalid Capp configuration: correct it. Configuration is valid but children are not created: the operator is failing to create them.
- **Many Capps at once** — Two different stalls:
  - The operator is **not processing** Capps (no new children): the operator is stuck or down.
  - The operator **is processing**, but every Capp has the same Ready reason: one shared cause.

---

## Service dependencies readiness

Ready is false because a child is not healthy — logging, hostname routing, DNS, certificate, volume, or an event trigger. The app itself may already be up. Each of those is a separate failure.

### Common causes

- **Logging** — the log pipeline or its destination is invalid or unreachable.
- **Hostname routing** — the custom hostname is not mapped to the Capp.
- **DNS** — the DNS record for the hostname has not been created or has not synced.
- **Certificate** — TLS for the hostname has not been issued.
- **Volume** — an NFS volume has not been provisioned.
- **Event trigger** — a Kafka or scheduled trigger is not ready.

### How to investigate

1. Read the Capp Ready **reason** and **message** — which child is blocking (logging, hostname, DNS, certificate, volume, or event trigger).
2. Look at that child. **Missing**: the operator never created it — operator logs. **Exists**: read that child's status message.
3. One Capp or many with the same reason? Many: the shared system for that child.

### How to resolve

- **Logging** — Wrong log settings on this Capp: correct them. Destination or logging system down: the log backend is unreachable.
- **Hostname routing** — Hostname on this Capp is wrong, not allowed, or already used: change it. Mapping failing for many Capps: hostname routing is broken, not a typo on one Capp.
- **DNS** — Record requested but not published: the DNS provider has not published the record.
- **Certificate** — Certificates not being issued: issuance is failing (issuer or CA).
- **Volume** — Wrong volume settings on this Capp: correct them. NFS or storage down: the storage backend is unavailable.
- **Event trigger** — Wrong Kafka or schedule settings on this Capp: correct them. Kafka or eventing down: the eventing backend is unavailable.
- **Feature not needed** — Remove it from the Capp so Ready no longer waits on it.

---

## Request routing latency

Requests take too long to **reach** the app. The Capp is already Ready. This is extra delay on the platform serving path.

### Common causes

- **Gateway** — the shared entry point is overloaded, or requests take a longer path (they queue in front of instances that are already running).
- **TLS** — HTTPS termination on the gateway is slow (all HTTPS traffic, not one Capp).
- **Concurrency** — this Capp accepts too few in-flight requests, so new ones wait.

### How to investigate

1. One Capp slow, or many?
2. Is the extra time **before** the request reaches the app, or **inside** the app? This alert is time before the app.
3. One Capp: this Capp's in-flight request limit (concurrency). Many: the shared gateway. If only HTTPS is slow: TLS on the gateway.

### How to resolve

- **Concurrency** — This Capp's in-flight request limit is too low: raise it so requests are not queued.
- **TLS** — All HTTPS traffic is slow: TLS termination on the gateway is slow.
- **Gateway** — The shared entry point is overloaded, unhealthy, or putting extra queueing in front of running instances.

---

## Scale lag

The Capp is already Ready and serving. Extra instances are slow to appear (or never appear) after load rises.

### Common causes

- **Max scale** — this Capp is already at its max instance count (including the platform max).
- **Scale settings** — this Capp's metric, target, or scale type does not raise the desired count under this load (request-based vs CPU/memory).
- **Autoscaler** — desired count is not moving for many Capps (shared autoscaler).

### How to investigate

1. Compare **desired** vs **actual** instance count.
2. Actual already at max scale: the cap.
3. Desired **not rising**: one Capp → max scale or scale settings (metric, target, type). Many Capps → shared autoscaler.
4. Desired **up**, actual **not**: extra instances are not running.

### How to resolve

- **Max scale** — This Capp is at max instances: raise max scale. If it is already at the platform max, that is the cap.
- **Scale settings** — Desired count does not rise under load: change this Capp's scale metric, target, or type. Request-based load often fits concurrency or RPS; CPU/memory fits resource-bound load.
- **Autoscaler** — Desired count stuck for many Capps: the shared autoscaler is not updating.
- **New instances not running** — Desired is up, actual is not: pull, schedule, or start.

---

## Activator latency

Requests are slow on the **scale-from-zero** path: the Capp has no running instances, so the request **waits in the activator** until an instance exists. This is queue time in the activator, not how long that instance takes to become Ready.

### Common causes

- **Scaled to zero** — this Capp is at zero instances, so every request waits in the activator.
- **Activator overloaded** — many Capps waking at once, or a burst of requests queued in the activator.
- **Activator workload** — activator instances not running or not healthy.

### How to investigate

1. Was this Capp at **zero instances** when the request was slow?
2. Are activator instances running and healthy? If **not**: its logs.
3. If activator instances **are** healthy: one Capp at zero, or many Capps / requests hitting the activator at once.

### How to resolve

- **Scaled to zero** — This Capp should not go to zero: raise min instances.
- **Activator overloaded** — Too many Capps or requests waking at once: the shared activator is saturated.

---

## Cold-start duration

The first request is slow because a **new instance** was requested and is not serving yet. Distinctive cause: the **image is not on the node** (first pull, often a large image). It can also still be pending or crashing.

### Common causes

- **Image not on the node** — first pull on that node, often a large image. Also: unreachable registry or bad credentials.
- **Pending or crashing** — the new instance cannot run, or it exits.

### How to investigate

1. The new instance: still **pulling** (image already on the node?), **pending**, or **crashing**?
2. One Capp or many? Many pulling: registry. One: this Capp's image or credentials.

### How to resolve

- **Image not on the node** — Large image: shrink it. Wrong credentials: correct them. Registry unreachable: the registry is blocking the pull.
- **Pending or crashing** — Pending: quota or cluster capacity. Crashing: fix the application.
