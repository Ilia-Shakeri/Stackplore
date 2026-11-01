#Requires -Version 7.4
[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'

# Refuse accidental workstation use before any filesystem or process operation.
if(-not $IsWindows -or $env:GITHUB_ACTIONS -cne 'true' -or $env:RUNNER_ENVIRONMENT -cne 'github-hosted'){
  throw 'hosted_processor.runner_required'
}
if($env:GITHUB_SHA -cnotmatch '\A[a-f0-9]{40}\z' -or [string]::IsNullOrWhiteSpace($env:RUNNER_TEMP)){
  throw 'hosted_processor.context_required'
}

$repository=(Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
$temporary=(Resolve-Path -LiteralPath $env:RUNNER_TEMP).Path
$private=Join-Path $temporary ('stackplore-profile-'+[guid]::NewGuid().ToString('N'))
$privateFull=[IO.Path]::GetFullPath($private)
if(-not $privateFull.StartsWith($temporary.TrimEnd('\')+'\',[StringComparison]::OrdinalIgnoreCase)){
  throw 'hosted_processor.private_path'
}
$null=New-Item -ItemType Directory -Path $privateFull
$outputDirectory=Join-Path $repository '.tmp'
$null=New-Item -ItemType Directory -Path $outputDirectory -Force
$publicReceipt=Join-Path $outputDirectory 'hosted-processor.json'
if(Test-Path -LiteralPath $publicReceipt){throw 'hosted_processor.receipt_exists'}
$launcher=Join-Path $PSScriptRoot 'local-processor.ps1'
$ownerReceipt=Join-Path $privateFull 'owner.json'
$archive=Join-Path $privateFull 'processor.zip'
$runtime=Join-Path $privateFull 'runtime'
$model=Join-Path $privateFull 'model.gguf'
$modelDigest='d2387ca2dbfee2ffabce7120d3770dadca0b293052bc2f0e138fdc940d9bc7b5'
$archiveDigest='aef737d26bffd08ceeb7be75f045bdef6d1ecc3187e0f4a1a3b0d878a03d2550'
$report=[ordered]@{
  schema_version=1
  source_revision=$env:GITHUB_SHA
  recorded_at=[DateTime]::UtcNow.ToString('o')
  status='failed'
  phase='archive'
  scope='Real bilingual protocol and application browser with synthetic Demo evidence; not general quality acceptance'
  archive_sha256=$archiveDigest
  model_sha256=$modelDigest
  model_revision='daeb8e2d528a760970442092f6bf1e55c3b659eb'
  model_license='Apache-2.0'
  model_bytes=1282439264
  context_tokens=20480
  threads=4
  cases=@()
  stopped=$false
}

function Receive-PinnedAsset([string]$Uri,[string]$Destination,[string]$Digest,[long]$Maximum,[long]$Exact=0){
  & curl.exe --fail --silent --location --proto '=https' --proto-redir '=https' --max-redirs 5 --retry 2 --max-time 300 --max-filesize $Maximum --output $Destination $Uri *> $null
  if($LASTEXITCODE -ne 0){throw 'hosted_processor.download_failed'}
  $item=Get-Item -LiteralPath $Destination -Force
  if($item.PSIsContainer -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -or $item.Length -lt 1048576 -or $item.Length -gt $Maximum -or ($Exact -ne 0 -and $item.Length -ne $Exact)){
    throw 'hosted_processor.asset_size'
  }
  if((Get-FileHash -LiteralPath $Destination -Algorithm SHA256).Hash.ToLowerInvariant() -cne $Digest){
    throw 'hosted_processor.asset_digest'
  }
}

try {
  Receive-PinnedAsset 'https://github.com/ggml-org/llama.cpp/releases/download/b11729/llama-b11729-bin-win-cpu-x64.zip' $archive $archiveDigest 268435456
  $report.phase='model'
  Receive-PinnedAsset 'https://huggingface.co/ggml-org/Qwen3-1.7B-GGUF/resolve/daeb8e2d528a760970442092f6bf1e55c3b659eb/Qwen3-1.7B-Q4_K_M.gguf?download=true' $model $modelDigest 1282439264 1282439264
  $report.phase='extract'
  $zip=[IO.Compression.ZipFile]::OpenRead($archive)
  try {
    if($zip.Entries.Count -ne 51){throw 'hosted_processor.archive_inventory'}
    foreach($entry in $zip.Entries){
      if($entry.FullName -cnotmatch '\A[A-Za-z0-9][A-Za-z0-9_.-]{0,127}\z' -or $entry.FullName.EndsWith('.') -or $entry.Length -lt 1 -or $entry.Length -gt 33554432){
        throw 'hosted_processor.archive_entry'
      }
    }
    [IO.Compression.ZipFileExtensions]::ExtractToDirectory($zip,$runtime)
  } finally {$zip.Dispose()}

  $report.phase='launch'
  $ready=(& $launcher -Action Start -Archive $archive -RuntimeDirectory $runtime -Model $model -ModelSHA256 $modelDigest -ReceiptPath $ownerReceipt -Threads 4) | ConvertFrom-Json
  if($ready.status -cne 'ready' -or $ready.template_sha256 -cnotmatch '\A[a-f0-9]{64}\z'){
    throw 'hosted_processor.start_failed'
  }
  $report['template_sha256']=$ready.template_sha256
  $report['build']=$ready.build
  $env:STACKPLORE_TEST_LOCAL_INFERENCE='true'
  $env:STACKPLORE_TEST_INFERENCE_ORIGIN='http://127.0.0.1:58102'
  $env:STACKPLORE_TEST_INFERENCE_ALIAS='sha256-'+$modelDigest
  $env:STACKPLORE_TEST_INFERENCE_BUILD=$ready.build
  $env:STACKPLORE_TEST_INFERENCE_TEMPLATE_SHA256=$ready.template_sha256
  $env:STACKPLORE_TEST_INFERENCE_CONTEXT_TOKENS='20480'
  $report.phase='protocol'
  $events=@(& go -C (Join-Path $repository 'apps/api') test ./internal/provider/inference -run '^TestPinnedLocal(Runtime|Evidence)Integration$' -count=1 -timeout=3m -json 2>$null)
  $goExit=$LASTEXITCODE
  $decoded=@($events | ForEach-Object {$_ | ConvertFrom-Json})
  foreach($name in @('TestPinnedLocalRuntimeIntegration/en','TestPinnedLocalRuntimeIntegration/fa','TestPinnedLocalRuntimeIntegration','TestPinnedLocalEvidenceIntegration/en','TestPinnedLocalEvidenceIntegration/fa','TestPinnedLocalEvidenceIntegration')){
    $finished=@($decoded | Where-Object {$_.Action -in @('pass','fail','skip') -and $_.PSObject.Properties.Name -contains 'Test' -and $_.Test -ceq $name})
    if($finished.Count -eq 1){
      $elapsed=0
      if($finished[0].PSObject.Properties.Name -contains 'Elapsed'){$elapsed=$finished[0].Elapsed}
      $report.cases+=@{name=$name;status=$finished[0].Action;elapsed_seconds=$elapsed}
    } else {$report.cases+=@{name=$name;status='missing_or_duplicate';elapsed_seconds=0}}
  }
  $report['evidence_probes']=@()
  foreach($event in $decoded){
    if($event.PSObject.Properties.Name -contains 'Output' -and $event.Output -match 'evidence_probe=(\{[^\r\n]+\})'){
      $flags=$Matches[1] | ConvertFrom-Json
      $report.evidence_probes+=@{locale=$flags.locale;backend_fault=$flags.backend_fault;gateway_fault=$flags.gateway_fault;input_tokens=$flags.input_tokens;output_tokens=$flags.output_tokens;input_match=$flags.input_match;complete=$flags.complete;json_valid=$flags.json_valid;sections=$flags.sections;citations_valid=$flags.citations_valid;identity_matches=$flags.identity_matches;identity_contained=$flags.identity_contained;facts_kind=$flags.facts_kind;text_bytes=$flags.text_bytes}
    }
  }
  $report['empty_probes']=@()
  foreach($event in $decoded){
    if($event.PSObject.Properties.Name -contains 'Output' -and $event.Output -match 'empty_probe=(\{[^\r\n]+\})'){
      $flags=$Matches[1] | ConvertFrom-Json
      $report.empty_probes+=@{locale=$flags.locale;gateway_fault=$flags.gateway_fault;sections=$flags.sections;gaps_kind=$flags.gaps_kind;empty_citations=$flags.empty_citations;text_exact=$flags.text_exact;text_contains=$flags.text_contains;text_bytes=$flags.text_bytes}
    }
  }
  if($goExit -ne 0){throw 'hosted_processor.protocol_failed'}
  if(@($decoded | Where-Object {$_.Action -in @('skip','fail')}).Count -ne 0 -or @($report.cases | Where-Object {$_.status -cne 'pass'}).Count -ne 0){throw 'hosted_processor.incomplete_probe'}
  $report.phase='application_browser'
  $browser=& (Join-Path $PSScriptRoot 'check-hosted-browser.ps1')
  if($browser.status -cne 'passed' -or -not $browser.postgres_stopped -or $browser.cases.Count -ne 2){
    throw 'hosted_processor.browser_failed'
  }
  $report['browser_cases']=$browser.cases
  $report['postgres_version']=$browser.postgres_version
  $report['postgres_stopped']=$browser.postgres_stopped
  $report.status='passed'
  $report.phase='complete'
} catch {
  $report.status='failed'
  $report['error_code']='hosted_processor.operation_failed'
  if($_.Exception.Message -cmatch '\Ahosted_(?:processor|browser)\.[a-z_]+\z'){$report['error_code']=$_.Exception.Message}
  throw 'hosted_processor.verification_failed'
} finally {
  foreach($name in @('LOCAL_INFERENCE','INFERENCE_ORIGIN','INFERENCE_ALIAS','INFERENCE_BUILD','INFERENCE_TEMPLATE_SHA256','INFERENCE_CONTEXT_TOKENS')){
    Remove-Item -LiteralPath ('Env:STACKPLORE_TEST_'+$name) -ErrorAction SilentlyContinue
  }
  try {
    if(Test-Path -LiteralPath $ownerReceipt){
      $owned=Get-Content -LiteralPath $ownerReceipt -Raw | ConvertFrom-Json
      if($owned.status -ceq 'ready'){
        $stopped=(& $launcher -Action Stop -ReceiptPath $ownerReceipt) | ConvertFrom-Json
        if($stopped.status -cne 'stopped' -or -not $stopped.owned){throw 'hosted_processor.stop_failed'}
        $report.stopped=$true
      } elseif($owned.PSObject.Properties.Name -contains 'owned_process_stopped' -and $owned.owned_process_stopped -eq $true){
        $report.stopped=$true
      }
    }
    if($report.status -ceq 'passed' -and -not $report.stopped){throw 'hosted_processor.stop_unproven'}
  } catch {
    $report.status='failed'
    $report['cleanup_error_code']='hosted_processor.cleanup_failed'
    throw 'hosted_processor.cleanup_failed'
  } finally {
    $metadataInvalid=$false
    $browserReceipt=Join-Path $outputDirectory 'hosted-browser.json'
    if(Test-Path -LiteralPath $browserReceipt){
      try {
        $browserMetadata=Get-Content -LiteralPath $browserReceipt -Raw | ConvertFrom-Json
        if($browserMetadata.source_revision -cne $env:GITHUB_SHA){throw 'hosted_processor.receipt_source'}
        $report['application_browser']=$browserMetadata
      } catch {
        $report.status='failed'
        $report['metadata_error_code']='hosted_processor.browser_receipt_invalid'
        $metadataInvalid=$true
      }
    }
    $report | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $publicReceipt -Encoding utf8NoBOM
    if($metadataInvalid){throw 'hosted_processor.browser_receipt_invalid'}
  }
}
