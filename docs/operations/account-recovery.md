# First-Run Setup and Account Recovery

This procedure covers the two operator-controlled account paths: protecting an
empty installation that is reachable beyond loopback, and recovering a local
administrator while the service is offline. There is no default account or
password.

## First setup on loopback

The default listener and public origin are loopback-only. Start the service,
open `/setup`, and create the first administrator. The transaction succeeds
only while the user table is empty and then closes network setup permanently.
Concurrent setup attempts cannot create another initial administrator.

## First setup beyond loopback

An empty installation whose listener or public origin is not loopback will not
start until a paired bootstrap proof and expiry are configured. Generate 32
random bytes outside shell history, encode them as canonical unpadded base64url,
and choose a canonical UTC expiry no more than one hour in the future. A
15-minute window is recommended.

PowerShell:

```powershell
$proofBytes = New-Object byte[] 32
$proofRng = [Security.Cryptography.RandomNumberGenerator]::Create()
try {
  $proofRng.GetBytes($proofBytes)
} finally {
  $proofRng.Dispose()
}
$env:STACKPLORE_SETUP_BOOTSTRAP_PROOF = [Convert]::ToBase64String($proofBytes).TrimEnd('=').Replace('+', '-').Replace('/', '_')
$env:STACKPLORE_SETUP_BOOTSTRAP_EXPIRES_AT = [DateTime]::UtcNow.AddMinutes(15).ToString("yyyy-MM-dd'T'HH:mm:ss'Z'", [Globalization.CultureInfo]::InvariantCulture)
[Array]::Clear($proofBytes, 0, $proofBytes.Length)
Remove-Variable proofBytes, proofRng
```

POSIX shell with the pinned Node.js runtime:

```sh
export STACKPLORE_SETUP_BOOTSTRAP_PROOF="$(node -e 'process.stdout.write(require("node:crypto").randomBytes(32).toString("base64url"))')"
export STACKPLORE_SETUP_BOOTSTRAP_EXPIRES_AT="$(node -e 'const expiry = new Date(Date.now() + 15 * 60 * 1000); expiry.setUTCMilliseconds(0); process.stdout.write(expiry.toISOString().replace(".000Z", "Z"))')"
```

Start the service in the same environment, open `/setup`, and enter the proof
from `STACKPLORE_SETUP_BOOTSTRAP_PROOF` in the operator-proof field. The server
keeps only a digest in its in-process gate. Successful setup consumes the gate
and closes setup in the database. Wrong, expired, missing, or replayed proof is
denied without copying the value to storage, logs, errors, or audit metadata.

After setup succeeds, remove both values and restart the service:

```powershell
Remove-Item Env:STACKPLORE_SETUP_BOOTSTRAP_PROOF -ErrorAction SilentlyContinue
Remove-Item Env:STACKPLORE_SETUP_BOOTSTRAP_EXPIRES_AT -ErrorAction SilentlyContinue
```

POSIX shell:

```sh
unset STACKPLORE_SETUP_BOOTSTRAP_PROOF STACKPLORE_SETUP_BOOTSTRAP_EXPIRES_AT
```

Do not put the proof in `.env`, source control, command arguments, tickets, or
chat. Deliver it only to the operator completing the first setup. Explicitly
configuring the pair also enables the proof gate on loopback.

## Offline administrator recovery

1. Stop the API process. The command requires exclusive ownership of the same
   data directory and refuses to run beside the server, backup, or restore.
2. Load the same `STACKPLORE_DATA_DIRECTORY` and
   `STACKPLORE_DATABASE_BUSY_TIMEOUT` configuration used by the server.
3. Verify the exact existing login name. A typo can create another
   administrator.
4. Feed one new password line through protected standard input. Never put a
   password in an argument, environment variable, temporary file, or shell
   history.

PowerShell hides the prompt, keeps the password out of command arguments and
history, and sends it to the child process through UTF-8 standard input. The
temporary output-encoding override is required for non-ASCII passwords on
Windows PowerShell 5.1:

```powershell
$recoverySecret = Read-Host 'New administrator password' -AsSecureString
$recoveryCredential = [PSCredential]::new('local-recovery', $recoverySecret)
$previousOutputEncoding = $OutputEncoding
$operatorExitCode = 1
try {
  $OutputEncoding = New-Object System.Text.UTF8Encoding($false)
  $recoveryCredential.GetNetworkCredential().Password |
    go -C apps/api run ./cmd/operator recover-admin --login-name operator --display-name "Primary Operator"
  $operatorExitCode = $LASTEXITCODE
} finally {
  $OutputEncoding = $previousOutputEncoding
  Remove-Variable recoveryCredential, recoverySecret, previousOutputEncoding
}
if ($operatorExitCode -ne 0) {
  throw "Administrator recovery failed with exit code $operatorExitCode"
}
```

An interactive POSIX terminal shell can disable terminal echo for the read and
restore it even when the command is interrupted:

```sh
old_tty_state=$(stty -g) || exit 1
trap 'stty "$old_tty_state"; unset recovery_password' 0
trap 'exit 1' 1 2 15
printf 'New administrator password: ' >&2
stty -echo || exit 1
IFS= read -r recovery_password || exit 1
stty "$old_tty_state" || exit 1
printf '\n' >&2
printf '%s\n' "$recovery_password" | \
  go -C apps/api run ./cmd/operator recover-admin \
    --login-name operator --display-name 'Primary Operator'
operator_exit_code=$?
unset recovery_password
trap - 0 1 2 15
test "$operator_exit_code" -eq 0
```

The password must be 12 through 128 UTF-8 bytes. Success prints exactly
`administrator recovery complete`. For an existing login, recovery enables and
promotes the account, updates its display name, replaces the password, advances
its authorization revision, and revokes all sessions and recent-auth grants.
For an absent login, it creates a new administrator. On an empty installation,
it also closes first-run setup. It never reopens setup.

Recovery writes a bounded local-operator audit event. Restart the service,
require `/readyz` to return `200`, sign in with the recovered account, and
change the temporary password through the user menu. That change rotates the
current session and revokes every other session and recent-auth grant for the
account.
