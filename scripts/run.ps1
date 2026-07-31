param(
  [Parameter(Mandatory = $true, Position = 0)]
  [ValidateSet(
    'bootstrap',
    'dev',
    'verify',
    'test',
    'test-api',
    'test-web',
    'build',
    'check-locales',
    'lint',
    'typecheck',
    'format',
    'format-check'
  )]
  [string]$Task
)

$ErrorActionPreference = 'Stop'

$scriptNames = @{
  'bootstrap' = 'bootstrap'
  'dev' = 'dev'
  'verify' = 'verify'
  'test' = 'test'
  'test-api' = 'test:api'
  'test-web' = 'test:web'
  'build' = 'build'
  'check-locales' = 'check:locales'
  'lint' = 'lint'
  'typecheck' = 'typecheck'
  'format' = 'format'
  'format-check' = 'format:check'
}

$repositoryRoot = Split-Path -Parent $PSScriptRoot
$exitCode = 0

Push-Location -LiteralPath $repositoryRoot
try {
  if ($Task -eq 'bootstrap') {
    & node scripts/bootstrap.mjs
  }
  else {
    & corepack pnpm run $scriptNames[$Task]
  }

  $exitCode = $LASTEXITCODE
}
finally {
  Pop-Location
}

exit $exitCode
