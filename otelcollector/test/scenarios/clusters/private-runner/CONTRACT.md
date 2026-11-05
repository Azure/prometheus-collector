# Private runner finalizer

Cleanup only: no provisioning, admission workflow, readiness audit, registration,
IAM changes or pipeline queuing. No live cleanup or mutation has been performed.

## Commands and receipt

From the repository root; Python standard library only:

```powershell
python -B otelcollector\test\scenarios\clusters\private-runner\test_runner.py
python -B otelcollector\test\scenarios\clusters\private-runner\test_runner.py --mutants
# Future operator use after independent host commissioning:
python -B otelcollector\test\scenarios\clusters\private-runner\runner.py --receipt owned-receipt.json --apply
```

`fixture()` in `test_runner.py` supplies the receipt format with **fake IDs, never
for live use**: version 1, UUID `run`, `name=pr-<UUID hex>`, UTC `started`/`deadline`
at most 12h apart, owned `resourceGroup`/`vnet`, exact `resources` IDs, native
`pool`/`queue`/`agent` IDs, `completed=[]`. Resources: `vm`, `disk`, `nic`, `nsg`,
`subnet`, optionally **both** `nat`/`ip`, with run-specific names in the fixed
subscription. Root resources need matching `privateRunnerRun`/
`privateRunnerDeadline` tags. Subnet guard: exact ID, `10.227.0.0/24`, owned NSG.
VM disk/NIC must match the receipt with `deleteOption=Detach`, no data disks,
preventing cascading deletion of unowned attachments.

Finalizer checks ownership/inventory, extra agents and authorization (only 979 or
already revoked); pool 66/queue 294 are forbidden. It unregisters the agent,
revokes 979 and verifies no authorized pipelines, deletes queue/pool, then exact
owned resources. No VNet, RG, shared NAT or role deletion. Each deletion rechecks
ownership, sends available ARM ETags, polls for 404 and journals progress.
Success requires all IDs and run-tagged RG inventory absent. Changes/failures/
leftovers fail closed; partial cleanup and failed journal writes are retryable.

Serialize execution: ADO deletion is not conditional; ARM ETags need live proof.
Journal uses exclusive adjacent `.next`, fsync and atomic replacement; leftover
`.next` needs reconciliation. Cleanup opens 30m before `deadline`, never before
`started`. External dispatch must allow retries/deletion within 12h; this script
is not a scheduler and cannot guarantee deletion during provider outages.

## Coordinator action: external host/store and minimum permissions

Authorize an **existing independent durable host and protected receipt store**,
or obtain separate approval for new resources. Discovered Automation account
existence is **not authorization**. Host must survive runner/pipeline loss,
schedule cleanup, retry and alert. Store must exclude runner/pipeline access;
host needs receipt read/update/atomic replacement and exclusive execution,
not receipt deletion. Local storage needs directory/file read/write/rename
including `.next`; cloud-store rights depend on the actual selected store.

ARM actions used (verified through read-only provider metadata):

| Scope | Actions |
|---|---|
| Owned RG inventory | `Microsoft.Resources/subscriptions/resources/read`, assigned at RG, **not subscription** |
| Exact VM/disk | `Microsoft.Compute/virtualMachines/read`, `/delete`; `Microsoft.Compute/disks/read`, `/delete` |
| Exact NIC/NSG/subnet | `Microsoft.Network/networkInterfaces/read`, `/delete`; `Microsoft.Network/networkSecurityGroups/read`, `/delete`; `Microsoft.Network/virtualNetworks/subnets/read`, `/delete` |
| Only if owned | `Microsoft.Network/natGateways/read`, `/delete`; `Microsoft.Network/publicIPAddresses/read`, `/delete` |

Parent owns RG RBAC assignment/scoping. No ARM create/write, Run Command, VM login,
role-assignment or VNet-delete action is used. No storage grant unless selected.
ADO host identity needs **Administrator on the dedicated organization pool** and
**Administrator on its project queue**, in `https://dev.azure.com/github-private`,
project `2d36c31d-2f89-409f-9a3e-32e4e9699840`: read/remove agents, delete pool/queue,
read/revoke resource pipeline permissions. No org-wide admin or pipeline-edit/
queue-build right. API scopes `vso.agentpools_manage`/`vso.pipelineresources_manage`
do not substitute for object ACLs.

Adapter uses host-owned Azure CLI authentication, in-memory ARM/ADO tokens
(ADO resource `499b84ac-1321-427f-aa17-267ca6975798`), no copied-token/PAT input,
secret/provider-body logging or redirects. Renewable host authentication remains
required; an expiring interactive session is not durable unattended auth.

## Separate administrator/bootstrap action

ADO administrator creates dedicated self-hosted pool/project queue, restricts
pipeline access to 979 and grants registering user pool-specific **Administrator**:
Organization settings → Agent pools → dedicated pool → Security.
Agent >=3.227.1 supports `config.sh` auth type `AAD`; authorized user completes
device-code sign-in through a separately approved interactive bootstrap channel.
Do not copy credentials or relay codes through Run Command logs.
[Microsoft procedure](https://learn.microsoft.com/en-us/azure/devops/pipelines/agents/device-code-flow-agent-registration?view=azure-devops).

Host/store/channel remain uncommissioned. Before use, verify approved VM/disk
bounds, owned network/collisions/private path, no runner MI, no VM public IP and
no inbound SSH. **SSH public keys are not prohibited.** HTTPS egress must preserve
platform DNS/fabric reachability; this cleanup script does not audit readiness.
Prove external cleanup with guest stopped and interrupted execution.

## Verification

**20 offline tests passed; all 8 source mutants killed**: ownership, deadline,
only-979, unregister, revoke, owned delete, journaling, cascade safety.
Partial retry/persistence/absence tests preserved; syntax/whitespace checks passed.
CLI exposes cleanup only. No live lifecycle/durability claim. Prior native-auth
read verified pipeline 979; owned-target routes/private path and effective pool
administration remain unproven.
