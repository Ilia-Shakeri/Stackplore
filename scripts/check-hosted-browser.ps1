#Requires -Version 7.4
[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
if(-not $IsWindows -or $env:GITHUB_ACTIONS -cne 'true' -or $env:RUNNER_ENVIRONMENT -cne 'github-hosted'){
  throw 'hosted_browser.runner_required'
}
$repository=(Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
if(Test-Path -LiteralPath (Join-Path $repository '.tmp/hosted-browser.json')){throw 'hosted_browser.receipt_exists'}
$temporary=(Resolve-Path -LiteralPath $env:RUNNER_TEMP).Path
$data=[IO.Path]::GetFullPath((Join-Path $temporary ('stackplore-browser-pg-'+[guid]::NewGuid().ToString('N'))))
if(-not $data.StartsWith($temporary.TrimEnd('\')+'\',[StringComparison]::OrdinalIgnoreCase)){
  throw 'hosted_browser.private_path'
}
$pgBin=Join-Path $env:ProgramFiles 'PostgreSQL\17\bin'
foreach($name in @('initdb.exe','pg_ctl.exe','postgres.exe','psql.exe')){
  if(-not (Test-Path -LiteralPath (Join-Path $pgBin $name))){throw 'hosted_browser.postgres_required'}
  $item=Get-Item -LiteralPath (Join-Path $pgBin $name) -Force
  if($item.PSIsContainer -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)){
    throw 'hosted_browser.postgres_required'
  }
}
if(@(Get-NetTCPConnection -LocalPort 59639 -State Listen -ErrorAction SilentlyContinue).Count -ne 0){
  throw 'hosted_browser.port_occupied'
}
$originalPath=$env:PATH
$owner=$null
$ownerTicks=0L
$started=$false
$summary=@{schema_version=1;source_revision=$env:GITHUB_SHA;status='failed';phase='database';postgres_stopped=$false;cases=@();journeys=@();database_metadata=@()}
try {
  $env:PATH=$pgBin+';'+$originalPath
  $version=(& (Join-Path $pgBin 'postgres.exe') --version) -join ''
  if($LASTEXITCODE -ne 0 -or $version -cnotmatch '\Apostgres \(PostgreSQL\) 17\.[0-9]+'){
    throw 'hosted_browser.postgres_version'
  }
  $summary.postgres_version=$version
  & (Join-Path $pgBin 'initdb.exe') -D $data -U stackplore_hosted --auth=trust --encoding=UTF8 --no-locale *> $null
  if($LASTEXITCODE -ne 0){throw 'hosted_browser.init_failed'}
  $control=Start-Process -FilePath (Join-Path $pgBin 'pg_ctl.exe') -ArgumentList @('start','-D',('"'+$data+'"'),'-l',('"'+(Join-Path $data 'server.log')+'"'),'-o','"-h 127.0.0.1 -p 59639"','-w','-t','60') -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $data 'start.out') -RedirectStandardError (Join-Path $data 'start.err')
  $control.WaitForExit()
  if($control.ExitCode -ne 0){throw 'hosted_browser.start_failed'}
  $started=$true
  $ownerID=[int]([IO.File]::ReadAllLines((Join-Path $data 'postmaster.pid'))[0])
  $owner=Get-Process -Id $ownerID
  $ownerTicks=$owner.StartTime.ToUniversalTime().Ticks
  $env:SP_BROWSER_ADMIN_URL='postgresql://stackplore_hosted@127.0.0.1:59639/postgres?sslmode=disable'
  $env:SP_BROWSER_MODES='normal-en,normal-fa'
  $env:SP_BROWSER_REAL_ASSISTANT='true'
  $env:SP_BROWSER_WORK_ROOT=$temporary
  $summary.phase='web_build'
  & corepack.cmd pnpm --dir $repository run build:web *> $null
  if($LASTEXITCODE -ne 0){throw 'hosted_browser.web_build_failed'}
  & node (Join-Path $repository 'scripts/embed-web-assets.mjs') *> $null
  if($LASTEXITCODE -ne 0){throw 'hosted_browser.embed_failed'}
  $summary.phase='browser'
  & node (Join-Path $repository 'scripts/browser-gate.mjs') *> $null
  $browserExit=$LASTEXITCODE
  foreach($locale in @('en','fa')){
    $aggregatePath=Join-Path $repository ('.tmp/hosted-database-normal-'+$locale+'.json')
    if(Test-Path -LiteralPath $aggregatePath){
      $aggregate=Get-Content -LiteralPath $aggregatePath -Raw | ConvertFrom-Json
      if($aggregate.source_revision -cne $env:GITHUB_SHA){throw 'hosted_browser.receipt_source'}
      $summary.database_metadata+=@{locale=$locale;snapshot=$aggregate}
    }
    foreach($role in @('administrator','viewer')){
      $journeyPath=Join-Path $repository ('.tmp/hosted-live-'+$locale+'-'+$role+'.json')
      if(Test-Path -LiteralPath $journeyPath){
        $journey=Get-Content -LiteralPath $journeyPath -Raw | ConvertFrom-Json
        if($journey.source_revision -cne $env:GITHUB_SHA){throw 'hosted_browser.receipt_source'}
        $summary.journeys+=@{locale=$locale;role=$role;admissions=$journey.admissions;admission_status=$journey.admission_status;run_state=$journey.run_state;content_state=$journey.content_state;resource_name_supported=$journey.resource_name_supported;citation_current=$journey.citation_current;cleared=$journey.cleared;view_state=$journey.view_state}
      }
    }
    $file=Join-Path $repository ('.tmp/browser-results-normal-'+$locale+'.json')
    if(Test-Path -LiteralPath $file){
      $result=Get-Content -LiteralPath $file -Raw | ConvertFrom-Json
      $passed=$result.stats.expected -eq 1 -and $result.stats.unexpected -eq 0 -and $result.stats.flaky -eq 0 -and $result.stats.skipped -eq 0
      $summary.cases+=@{locale=$locale;roles=2;status=$(if($passed){'pass'}else{'fail'});expected=$result.stats.expected;unexpected=$result.stats.unexpected;flaky=$result.stats.flaky;skipped=$result.stats.skipped}
    } else {
      $summary.cases+=@{locale=$locale;roles=0;status='missing'}
    }
  }
  if($browserExit -ne 0){throw 'hosted_browser.journey_failed'}
  if(@($summary.cases | Where-Object {$_.status -cne 'pass'}).Count -ne 0){throw 'hosted_browser.incomplete_journey'}
  $summary.status='passed'
  $summary.phase='complete'
} catch {
  $summary['error_code']='hosted_browser.operation_failed'
  if($_.Exception.Message -cmatch '\Ahosted_browser\.[a-z_]+\z'){$summary['error_code']=$_.Exception.Message}
  throw 'hosted_browser.verification_failed'
} finally {
  foreach($name in @('ADMIN_URL','MODES','REAL_ASSISTANT','WORK_ROOT')){
    Remove-Item -LiteralPath ('Env:SP_BROWSER_'+$name) -ErrorAction SilentlyContinue
  }
  $env:PATH=$originalPath
  try {
    if($started){
      $currentID=[int]([IO.File]::ReadAllLines((Join-Path $data 'postmaster.pid'))[0])
      $current=Get-Process -Id $currentID
      if($null -eq $owner -or $currentID -ne $owner.Id -or $current.StartTime.ToUniversalTime().Ticks -ne $ownerTicks -or -not [string]::Equals($current.Path,(Join-Path $pgBin 'postgres.exe'),[StringComparison]::OrdinalIgnoreCase)){
        throw 'hosted_browser.ownership_changed'
      }
      & (Join-Path $pgBin 'pg_ctl.exe') stop -D $data -m fast -w -t 30 *> $null
      if($LASTEXITCODE -ne 0){throw 'hosted_browser.stop_failed'}
      $summary.postgres_stopped=$true
    }
  } catch {
    $summary.status='failed'
    $summary['cleanup_error_code']='hosted_browser.cleanup_failed'
    throw 'hosted_browser.cleanup_failed'
  } finally {
    $summary | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $repository '.tmp/hosted-browser.json') -Encoding utf8NoBOM
  }
}
return $summary
