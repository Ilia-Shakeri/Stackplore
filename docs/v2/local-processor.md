# Smart Assistant — Windows local processor operator guide

[فارسی](local-processor.fa.md) · [Profile contract](local-profile.md) ·
[Implementation status](implementation.md)

This guide describes a native operator tool, not an enabled product feature or
a completed roadmap stage. The maintainer authorized local installation and
startup. Runtime acceptance still requires exact-source integration receipts.
The previous automatic-review launch rejection remains a failed attempt. If a
new invocation is rejected, preserve that result; do not switch launch methods.

Use PowerShell **7.4 or newer** on Windows. Run this tool only as the trusted
host operator, under a dedicated local account where practical. The process
list and its private receipt expose local file paths to that account. Other
local programs can reach the loopback service; this is a trusted workstation
boundary, not isolation between hostile local users. No firewall or public
listener is added.

## Approved input files

The tool accepts an existing, operator-verified CPU x64 installation:

| Pin                    | Required value                                                       |
| ---------------------- | -------------------------------------------------------------------- |
| Release                | `b11729`, `llama-b11729-bin-win-cpu-x64.zip`                         |
| Archive SHA-256        | `aef737d26bffd08ceeb7be75f045bdef6d1ecc3187e0f4a1a3b0d878a03d2550`   |
| Source                 | `412a363c655b479279d3050d0d1c67a5733697b4`                           |
| Expected process build | `b11729-412a363c6`                                                   |
| Installation           | Exactly the archive's 51 flat files; no additional files or links    |
| Model                  | One explicit local `.gguf` file, independently approved full SHA-256 |

Obtain the archive from the [official pinned release](https://github.com/ggml-org/llama.cpp/releases/tag/b11729).
The existing private download was renamed `llama-b11729-win-cpu-x64.zip`;
its bytes and required digest are unchanged. The tool binds bytes, not the basename.
Acquire the model from its approved publisher, review its license and revision,
and verify its full file digest independently. The launcher does not download,
extract or select models. It checks GGUF version 3, bounded header counts, and a
16 MiB–8 GiB file bound; that bound is not a memory or quality guarantee.
The model alias is exactly `sha256-` plus the approved lowercase digest.

The initial disposable protocol probe uses `Qwen/Qwen3-0.6B-GGUF`, revision
`23749fefcc72300e3a2ad315e1317431b06b590a`, Apache-2.0, file
`Qwen3-0.6B-Q8_0.gguf`, **639,446,688 bytes**, SHA-256
`9465e63a22add5354d9bb4b99e90117043c7124007664907259bd16d043bb031`.
This identifies test input; it does not qualify that profile's answer quality.

All supplied paths are private, fully qualified local drive paths chosen by the
operator. Network paths, filesystem links and reparse ancestry are refused.
Browser input never supplies these paths. Before execution, the tool hashes the
archive, verifies every installed file against its matching archive entry, and
hashes the model. Read-only handles remain open through startup to prevent
replacement during verification and loading. Keep the installed directory and
model under operator control after the handles close.

## Check, start and stop

Run commands from the repository root. Substitute independently verified files
and the complete lowercase digest; keep the selected private directory outside
the repository and restrict its access. The parent directory must already exist.

```powershell
$privateRoot = 'D:\Stackplore-local-processor'
$inputs = @{
  Archive = Join-Path $privateRoot 'llama-b11729-win-cpu-x64.zip'
  RuntimeDirectory = Join-Path $privateRoot 'llama-b11729'
  Model = Join-Path $privateRoot 'approved-model.gguf'
  ModelSHA256 = '<independently-verified-64-character-lowercase-digest>'
}

# Default action: file validation only; no program or network request.
& .\scripts\local-processor.ps1 @inputs

# Explicit startup; use a new private receipt for every attempt.
$receipt = Join-Path $privateRoot ('processor-' + [guid]::NewGuid().ToString('N') + '.json')
& .\scripts\local-processor.ps1 @inputs -Action Start -ReceiptPath $receipt -Threads 2

# Stop only the process proven to belong to that exact receipt.
& .\scripts\local-processor.ps1 -Action Stop -ReceiptPath $receipt
```

`Start` refuses any existing listener on port **58102**, then starts only
`127.0.0.1:58102` in a hidden window. It permits two to four CPU threads, one
slot, 20,480 context tokens, quantized `q8_0` key/value caches and no offload.
The allowed browser origin is exactly `http://127.0.0.1:58102`, avoiding the
processor's default arbitrary-origin reflection; no authentication credential
is configured for this trusted-host-only probe.
Startup is bounded to 10–120 seconds, default 120. Every status read has a
two-second deadline, a 1 MiB body cap, no redirects and no ambient proxy.
The listener's owning PID is checked before requests; process identity, build,
model alias, template digest, context and slot count are checked before readiness.
A successful startup does not prove response quality, maximum-load latency or
the full 25-second application dispatch budget.

The fixed argument set disables the web interface, built-in execution mode,
interface proxy, slot endpoint, automatic model loading, projector loading,
prompt caches, context shifting and raw logging. Tool, media, router,
slot-saving and writable-properties options are never supplied. Inherited
processor, download, proxy and credential variables are removed in the child;
the parent shell remains unchanged. No shell command is built from model text.
These settings follow the [pinned server argument reference](https://github.com/ggml-org/llama.cpp/blob/412a363c655b479279d3050d0d1c67a5733697b4/tools/server/README.md).
This is a restricted startup configuration, not an operating-system sandbox.

The private receipt records file paths, hashes, PID, UTC process start ticks,
settings and safe outcomes. Do not commit or publish it. Standard output contains
only safe pins, counts, settings and fixed error codes; no prompt, answer, model
path or upstream error body. Publish a separately minimized, source-bound
integration receipt only after the actual tests run.

`Stop` requires the exact executable path, PID and process start time and verifies
the executable digest. A foreign or reused PID is refused. A startup failure
preserves its receipt and stops only a process whose ownership is proven.
No directory, model, archive, receipt or unrelated process is deleted. Do not
reuse a failed receipt: investigate its fixed code, retain the failure and use a
fresh receipt for a newly authorized attempt.

## Verification and stage boundary

```powershell
pwsh -NoProfile -File .\scripts\local-processor.test.ps1
```

The pure checks do not launch a process or use the network. They cover pins,
GGUF/header limits, archive names, fixed arguments, child-environment removal,
quoting and process ownership. They do not establish actual installation or
startup acceptance.

After an approved real startup, run the opt-in bilingual integration gate in the
[profile contract](local-profile.md#real-integration-gate). Bind its source,
archive/model/template hashes, actual build, hardware, counts and durations.
Keep ordinary product dispatch disabled until durable admission, shared privacy,
current authority and their own stage gates pass. This tool does not expose
an endpoint selector or file path through the product API.
