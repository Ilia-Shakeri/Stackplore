#Requires -Version 7.4
# Fetch checks repository, source, jobs and ZIP digest before bounded parsing.
# Local validation checks archive structure only; it proves no browser execution.
[CmdletBinding(DefaultParameterSetName='Fetch')]
param(
  [Parameter(Mandatory=$true,ParameterSetName='Fetch')][long]$RunId,
  [Parameter(Mandatory=$true,ParameterSetName='Fetch')][ValidatePattern('^[a-f0-9]{40}$')][string]$Revision,
  [Parameter(Mandatory=$true,ParameterSetName='Fetch')][string]$Destination,
  [Parameter(ParameterSetName='Fetch')][string]$DestinationRoot,
  [Parameter(Mandatory=$true)][ValidatePattern('\A2\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:-(?:alpha|beta|rc)\.(?:0|[1-9][0-9]*))?\z')][string]$Version,
  [ValidateSet('success','failure')][string]$ExpectedConclusion='success',
  [ValidatePattern('^v2-[a-z0-9-]{1,100}\.json$')][string]$ReceiptName='v2-final-browser-artifact.json',
  [Parameter(Mandatory=$true,ParameterSetName='ArchiveOnly')][string]$ValidateArchive
)
$ErrorActionPreference='Stop'
$repository='Ilia-Shakeri/Stackplore'
$repoRoot=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$receiptPath=Join-Path $repoRoot ('.tmp/'+$ReceiptName)
$destinationRoot=if([string]::IsNullOrWhiteSpace($DestinationRoot)){Join-Path $repoRoot '.tmp/browser-evidence'}else{[IO.Path]::GetFullPath($DestinationRoot)}
$destinationRoot=$destinationRoot.TrimEnd([IO.Path]::DirectorySeparatorChar,[IO.Path]::AltDirectorySeparatorChar)+[IO.Path]::DirectorySeparatorChar
$downloadDirectory=if($PSCmdlet.ParameterSetName -ceq 'Fetch'){[IO.Path]::GetFullPath($Destination)}else{$null}
$zipPath=if($null -ne $downloadDirectory){Join-Path $downloadDirectory 'browser-evidence.zip'}else{$null}
$token=$null
$credentialLines=$null
$handler=$null
$client=$null
$diagnostic=$ExpectedConclusion -ceq 'failure'
function Require([bool]$Condition,[string]$Reason) { if(-not $Condition){throw $Reason} }
function Is-True($Value) { return ($Value -is [bool] -and $Value) }
function Is-Number($Value) { return (($Value -is [long] -or $Value -is [int] -or $Value -is [double] -or $Value -is [decimal]) -and [double]::IsFinite([double]$Value)) }
function Is-Count($Value,[long]$Maximum) { return ((Is-Number $Value) -and $Value -ge 0 -and $Value -le $Maximum -and [Math]::Floor([double]$Value) -eq $Value) }
function Digest([string]$Path) { return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant() }
function Same-Names($Actual,$Expected,[string]$Reason) {
  $a=@($Actual | Sort-Object -CaseSensitive); $b=@($Expected | Sort-Object -CaseSensitive)
  Require ($a.Count -eq $b.Count) $Reason
  for($i=0;$i -lt $a.Count;$i++){Require ($a[$i] -ceq $b[$i]) $Reason}
}
function Required-CIJobs([string]$ProductVersion) {
  Require ($ProductVersion -cmatch '\A2\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:-(?:alpha|beta|rc)\.(?:0|[1-9][0-9]*))?\z') 'CI job policy requires a canonical product version'
  $required=@('code','deploy','compose-install','helm-install','docker-provider','kubernetes-provider')
  # Preserve the immutable six-job alpha.0 checkpoint; every later source adds the host tool gate.
  if($ProductVersion -cne '2.0.0-alpha.0'){$required+='processor-tool'}
  return $required
}
function Check-CIJobs($Listing,[string]$ProductVersion,[bool]$Diagnostic) {
  $required=@(Required-CIJobs $ProductVersion)
  $actual=@($Listing.jobs)
  Require ((Is-Count $Listing.total_count 100) -and $Listing.total_count -eq $required.Count -and $actual.Count -eq $required.Count) 'Final CI job inventory differs from the versioned gate policy'
  Same-Names @($actual | ForEach-Object {$_.name}) $required 'Final CI required job names differ'
  foreach($job in $actual){
    Require ($job.status -ceq 'completed') 'Final CI contains an incomplete job'
    $expected=if($Diagnostic -and $job.name -ceq 'code'){'failure'}else{'success'}
    Require ($job.conclusion -ceq $expected) 'Final CI job conclusion differs from the authorized diagnostic or success state'
  }
}
function Read-Bounded($Stream,[long]$Maximum,[Threading.CancellationToken]$Cancellation) {
  $memory=[IO.MemoryStream]::new()
  $buffer=[byte[]]::new(65536)
  try {
    while($true){
      $count=$Stream.ReadAsync($buffer,0,$buffer.Length,$Cancellation).GetAwaiter().GetResult()
      if($count -eq 0){break}
      Require ($memory.Length+$count -le $Maximum) 'Input exceeds bounded read'
      $memory.Write($buffer,0,$count)
    }
    return ,$memory.ToArray()
  } finally {$memory.Dispose()}
}
function Api-Json([string]$Suffix) {
  $request=[Net.Http.HttpRequestMessage]::new([Net.Http.HttpMethod]::Get,"https://api.github.com/repos/$repository/$Suffix")
  $request.Headers.Authorization=[Net.Http.Headers.AuthenticationHeaderValue]::new('Bearer',$token)
  $request.Headers.Accept.ParseAdd('application/vnd.github+json')
  $response=$null; $stream=$null
  $deadline=[Threading.CancellationTokenSource]::new([TimeSpan]::FromSeconds(45))
  try {
    $response=$client.SendAsync($request,[Net.Http.HttpCompletionOption]::ResponseHeadersRead,$deadline.Token).GetAwaiter().GetResult()
    Require ([int]$response.StatusCode -eq 200) 'Required artifact API read failed'
    $stream=$response.Content.ReadAsStreamAsync().GetAwaiter().GetResult()
    $bytes=Read-Bounded $stream 8388608 $deadline.Token
    return (ConvertFrom-Json -InputObject ([Text.UTF8Encoding]::new($false,$true).GetString($bytes)))
  } finally {if($null -ne $stream){$stream.Dispose()}; if($null -ne $response){$response.Dispose()}; $request.Dispose(); $deadline.Dispose()}
}
function Zip-Report($Zip,[string]$Name) {
  $matches=@($Zip.Entries | Where-Object {$_.FullName -ceq $Name})
  Require ($matches.Count -eq 1 -and $matches[0].Length -gt 0 -and $matches[0].Length -le 16777216) 'Selected browser report missing, duplicate or over 16MiB'
  $stream=$matches[0].Open()
  $deadline=[Threading.CancellationTokenSource]::new([TimeSpan]::FromSeconds(15))
  try {$bytes=Read-Bounded $stream 16777216 $deadline.Token} finally {$stream.Dispose();$deadline.Dispose()}
  Require ($bytes.Length -eq $matches[0].Length) 'Browser report entry length mismatch'
  $hash=[Security.Cryptography.SHA256]::Create()
  try {$digest=([BitConverter]::ToString($hash.ComputeHash($bytes))).Replace('-','').ToLowerInvariant()} finally {$hash.Dispose()}
  return @{document=(ConvertFrom-Json -InputObject ([Text.UTF8Encoding]::new($false,$true).GetString($bytes)));sha256=$digest;bytes=$bytes.Length;entry=$Name}
}
function Suite-Tests($Suites) {
  foreach($suite in $Suites){
    foreach($spec in $suite.specs){foreach($case in $spec.tests){$case}}
    Suite-Tests $suite.suites
  }
}
function Check-Report($Parsed,[string]$Mode,[int]$Expected) {
  $report=$Parsed.document
  $tests=@(Suite-Tests $report.suites)
  Require ($tests.Count -eq $Expected) 'Browser case count mismatch'
  foreach($field in @('expected','unexpected','flaky','skipped')){Require (Is-Count $report.stats.$field 1000) 'Browser report statistics invalid'}
  Require (@($report.errors).Count -le 128) 'Browser report error inventory exceeds bound'
  $safeCases=@()
  $index=0
  foreach($case in $tests){
    $index++
    Require ($case.expectedStatus -ceq 'passed' -and $case.status -cin @('expected','unexpected','flaky','skipped') -and @($case.results).Count -ge 1 -and @($case.results).Count -le 4) 'Browser case status or retry shape invalid'
    $last=@($case.results)[-1]
    Require ($last.status -cin @('passed','failed','timedOut','interrupted','skipped')) 'Browser result status invalid'
    $category='none'
    if($last.status -cne 'passed'){
      $messages=(@($last.errors | ForEach-Object {$_.message}) -join "`n")
      $category=if($messages -match '(?i)public worker control read'){ 'public_worker_control_read' } elseif($messages -match '(?i)Reload to update'){ 'worker_update_prompt' } elseif($messages -match '(?i)public worker|serviceWorker|cacheNames|stackplore-public-shell|navigations'){ 'worker_update_state' } elseif($messages -match '(?i)timeout|timed out'){ 'browser_timeout' } elseif($messages -match '(?i)expect'){ 'browser_expectation' } else { 'browser_failure' }
    }
    $safeCases += [ordered]@{case=$index;status=$last.status;classification=$category;attempts=@($case.results).Count;error_count=@($last.errors).Count}
    if(-not $diagnostic){Require ($case.status -ceq 'expected' -and @($case.results).Count -eq 1 -and $last.status -ceq 'passed' -and (Is-Count $last.retry 0) -and @($last.errors).Count -eq 0) 'Browser case did not pass on its sole attempt'}
  }
  if(-not $diagnostic){Require ($report.stats.expected -eq $Expected -and $report.stats.unexpected -eq 0 -and $report.stats.flaky -eq 0 -and $report.stats.skipped -eq 0 -and @($report.errors).Count -eq 0) 'Browser report did not pass'}
  return @{summary=[ordered]@{mode=$Mode;entry=$Parsed.entry;cases=$tests.Count;passed=$report.stats.expected;unexpected=$report.stats.unexpected;flaky=$report.stats.flaky;skipped=$report.stats.skipped;report_error_count=@($report.errors).Count;case_statuses=$safeCases;bytes=$Parsed.bytes;sha256=$Parsed.sha256};tests=$tests}
}
function Worker-Metrics($Tests) {
  $attachments=@($Tests | ForEach-Object {$_.results} | ForEach-Object {$_.attachments} | Where-Object {$_.name -ceq 'public-worker-control-read'})
  if($diagnostic -and $attachments.Count -eq 0){return [ordered]@{status='not_present';attempts=@()}}
  Require ($attachments.Count -eq 1 -and $attachments[0].contentType -ceq 'application/json' -and $attachments[0].body -is [string] -and $attachments[0].body.Length -le 65536) 'Unique inline public worker attachment missing'
  $raw=[Convert]::FromBase64String($attachments[0].body)
  Require ($raw.Length -le 16384) 'Public worker attachment exceeds bound'
  $metrics=[Text.UTF8Encoding]::new($false,$true).GetString($raw) | ConvertFrom-Json
  $attempts=@($metrics.attempts)
  Require ((Is-Count $metrics.deadlineMs 15000) -and $metrics.deadlineMs -eq 15000 -and (Is-Count $metrics.maximumAttempts 4) -and $metrics.maximumAttempts -eq 4 -and $attempts.Count -ge 1 -and $attempts.Count -le 4) 'Worker attachment deadline or retry count mismatch'
  $safe=@()
  for($i=0;$i -lt $attempts.Count;$i++){
    $entry=$attempts[$i]
    Require ((Is-Count $entry.attempt 4) -and $entry.attempt -eq $i+1 -and (Is-Count $entry.status 599)) 'Worker attachment sequence or status invalid'
    if($diagnostic){
      Require ($entry.status -in @(200,429,400,401,403,404,500,502,503) -and $entry.mime -cin @('text/javascript','application/javascript','text/plain','other')) 'Worker attachment diagnostic values outside safe set'
      $item=[ordered]@{attempt=$entry.attempt;status=$entry.status;mime=$entry.mime}
      foreach($field in @('retryDelayMs','bytes','cacheMarkers')){if($null -ne $entry.$field){Require (Is-Count $entry.$field 16777216) 'Worker attachment numeric diagnostic outside bound';$item[$field]=$entry.$field}}
      $safe += $item
    } elseif($i -eq $attempts.Count-1){
      Require ($entry.status -eq 200 -and $entry.mime -cin @('text/javascript','application/javascript') -and (Is-Count $entry.bytes 65536) -and $entry.bytes -gt 0 -and (Is-Count $entry.cacheMarkers 1) -and $entry.cacheMarkers -eq 1) 'Worker attachment lacks successful bounded JavaScript result'
      $safe += [ordered]@{attempt=$entry.attempt;status=$entry.status;mime=$entry.mime;bytes=$entry.bytes;cache_markers=$entry.cacheMarkers}
    } else {
      Require ($entry.status -eq 429 -and $entry.mime -cin @('text/plain','text/javascript','application/javascript','other') -and (Is-Count $entry.retryDelayMs 5000)) 'Worker attachment contains unexpected retry outcome'
      $safe += [ordered]@{attempt=$entry.attempt;status=$entry.status;mime=$entry.mime;retry_delay_ms=$entry.retryDelayMs}
    }
  }
  return [ordered]@{deadline_ms=$metrics.deadlineMs;maximum_attempts=$metrics.maximumAttempts;attempts=$safe}
}
function Public-HttpFailures($Tests) {
  $safe=@();$index=0
  foreach($case in $Tests){
    $index++
    foreach($attachment in @($case.results | ForEach-Object {$_.attachments} | Where-Object {$_.name -ceq 'public-http-statuses'})){
      Require ($attachment.contentType -ceq 'application/json' -and $attachment.body -is [string] -and $attachment.body.Length -le 65536) 'Public HTTP attachment shape invalid'
      $raw=[Convert]::FromBase64String($attachment.body)
      Require ($raw.Length -le 32768) 'Public HTTP attachment exceeds bound'
      $statuses=@([Text.UTF8Encoding]::new($false,$true).GetString($raw) | ConvertFrom-Json)
      Require ($statuses.Count -le 128) 'Public HTTP status inventory exceeds bound'
      $entries=@()
      foreach($status in $statuses){Require ($status.component -is [string] -and $status.component -cmatch '\A[a-z-]{1,64}\z' -and (Is-Count $status.status 599) -and $status.status -ge 100) 'Public HTTP diagnostic values invalid';$entries+=[ordered]@{component=$status.component;status=$status.status}}
      $safe += [ordered]@{case=$index;statuses=$entries}
    }
  }
  return ,$safe
}
function Load-Metrics($Zip) {
  $parsed=Zip-Report $Zip "docs/evidence/$Version/browser-measurements.json"
  $load=$parsed.document.concurrentReads
  foreach($field in @('requests','parallel','rounds','minimumRoundSpacingMs','setupRateBucketRefillMs','setupNavigationRefills','setupNavigationRefillMs','initialRateBucketRefillMs','successfulResponses')){Require (Is-Count $load.$field 10000) 'Current load workload metadata invalid'}
  Require ($load.requests -eq 80 -and $load.parallel -eq 8 -and $load.rounds -eq 10 -and $load.minimumRoundSpacingMs -eq 600 -and $load.setupRateBucketRefillMs -eq 2500 -and $load.setupNavigationRefills -eq 4 -and $load.setupNavigationRefillMs -eq 2500 -and $load.initialRateBucketRefillMs -eq 2500 -and $load.setupExcludedFromReadTiming -is [bool] -and $load.setupExcludedFromReadTiming -and $load.successfulResponses -eq 80 -and $load.envelopesValidated -is [bool] -and $load.envelopesValidated) 'Current bounded load evidence lacks exact setup or workload gates'
  Require ((Is-Number $load.elapsedMs) -and (Is-Number $load.medianMs) -and (Is-Number $load.p95Ms) -and $load.elapsedMs -ge 6000 -and $load.elapsedMs -le 180000 -and $load.medianMs -ge 0 -and $load.p95Ms -ge $load.medianMs -and $load.p95Ms -le 10000) 'Current load timing values outside expected local fixture bounds'
  $update=$parsed.document.update
  Require ($update.explicitAcceptance -is [bool] -and $update.explicitAcceptance) 'Public worker update acceptance evidence missing'
  Same-Names @($update.publicPaths) @('/brand/stackplore-favicon-64.png','/offline.css','/offline.html','/offline.js') 'Public worker cache inventory differs from allowed shell'
  return [ordered]@{entry=$parsed.entry;sha256=$parsed.sha256;requests=$load.requests;parallel=$load.parallel;rounds=$load.rounds;minimum_round_spacing_ms=$load.minimumRoundSpacingMs;setup_navigation_refills=$load.setupNavigationRefills;setup_refill_ms=$load.setupNavigationRefillMs;setup_excluded_from_read_timing=$load.setupExcludedFromReadTiming;successful_responses=$load.successfulResponses;envelopes_validated=$load.envelopesValidated;elapsed_ms=$load.elapsedMs;median_ms=$load.medianMs;p95_ms=$load.p95Ms;public_worker_update_acceptance=$update.explicitAcceptance;public_cache_entries=@($update.publicPaths).Count;scope='Bounded fixture observations declared in this archive; no production capacity claim.'}
}

function Profile-Metrics($Document) {
  Require ($Document.version -ceq $Version) 'Browser measurements version mismatch'
  $expectedProfiles=@()
  foreach($locale in @('en','fa')){foreach($width in @(360,1280)){foreach($theme in @('light','dark')){$expectedProfiles+="$locale-$width-$theme"}}}
  $actualProfiles=@($Document.PSObject.Properties.Name | Where-Object {$_ -cmatch '^[a-z]{2}-[0-9]+-(?:light|dark)$'})
  Same-Names $actualProfiles $expectedProfiles 'Expected exactly eight browser profiles'
  $routes=@('/overview','/resources','/topology','/logs','/events','/alerts','/notifications','/search')
  $environmentProfiles=@()
  foreach($key in $expectedProfiles){
    $profile=$Document.$key
    Require ($profile.version -ceq $Version -and $profile.chromium -is [string] -and $profile.chromium.Length -ge 1 -and $profile.chromium.Length -le 64) 'Browser profile version or browser identity missing'
    $measurements=@($profile.measurements)
    Same-Names @($measurements | ForEach-Object {$_.route}) $routes 'Browser profile must measure all eight fixed routes'
    foreach($measurement in $measurements){
      Require ((Is-Number $measurement.routeReadyMs) -and $measurement.routeReadyMs -ge 0 -and $measurement.routeReadyMs -lt 30000 -and (Is-True $measurement.reducedMotion) -and $measurement.normalMotionDuration -ceq '1ms') 'Browser route timing or reduced motion assertion missing'
    }
    foreach($field in @('keyboard','accessibility','viewport','fixedNavigation','focusReturn')){Require (Is-True $profile.commandPalette.$field) 'Browser palette evidence missing'}
    Require (@($profile.httpFailures).Count -eq 0) 'Browser profile contains HTTP failures'
    $selectorProperty=$profile.PSObject.Properties['environmentSelector']
    if($key.EndsWith('-light',[StringComparison]::Ordinal)){
      Require ($null -ne $selectorProperty) 'Light profile environment selector proof missing'
      $selector=$selectorProperty.Value
      foreach($field in @('lazyReads','sourceBound','keyboard','focusReturn','accessibility','viewport')){Require (Is-True $selector.$field) 'Environment selector required behavior missing'}
      Require ((Is-Count $selector.environmentReads 2) -and $selector.environmentReads -eq 2 -and $selector.mobileDrawer -is [bool] -and $selector.mobileDrawer -eq ($key -cmatch '-360-')) 'Environment selector read budget or mobile drawer proof mismatch'
      $environmentProfiles += [ordered]@{profile=$key;environment_reads=2;lazy_reads=$true;source_bound=$true;keyboard=$true;focus_return=$true;accessibility=$true;viewport=$true;mobile_drawer=$selector.mobileDrawer}
    } else {Require ($null -eq $selectorProperty) 'Dark profile must not claim an unexecuted environment selector journey'}
  }
  Require ($environmentProfiles.Count -eq 4 -and @($environmentProfiles | Where-Object {$_.mobile_drawer}).Count -eq 2) 'Expected four environment journeys and two mobile drawer checks'
  Require ((Is-True $Document.offline.keyboardRetry) -and (Is-True $Document.offline.privateContentAbsent) -and (Is-True $Document.installability.chromiumProtocolChecked) -and (Is-True $Document.installability.regularProfile) -and @($Document.installability.errors).Count -eq 0 -and @($Document.installability.httpFailures).Count -eq 0) 'Browser offline or installability evidence missing'
  Same-Names @($Document.offline.selectedThemeOverridesDevice) @('light','dark') 'Offline selected theme must override both opposite device themes'
  Same-Names @($Document.offline.publicPaths) @('/brand/stackplore-favicon-64.png','/offline.css','/offline.html','/offline.js') 'Offline public cache inventory mismatch'
  return [ordered]@{profiles=8;routes_per_profile=8;route_measurements=64;command_palette_profiles=8;environment_selector_profiles=$environmentProfiles;offline_selected_theme_overrides_device=@($Document.offline.selectedThemeOverridesDevice);scope='Browser fixture measurements declared in this archive; no customer deployment or capacity claim.'}
}

function Browser-Metrics($Zip) {
  $parsed=Zip-Report $Zip "docs/evidence/$Version/browser-measurements.json"
  $profiles=Profile-Metrics $parsed.document
  $prefix="docs/evidence/$Version/screenshots/"
  $expected=@()
  foreach($locale in @('en','fa')){foreach($width in @(360,1280)){foreach($theme in @('light','dark')){foreach($component in @('overview','topology','command-palette','about')){$expected+="$prefix$component-$locale-$width-$theme.png"}}}}
  $images=@($Zip.Entries | Where-Object {$_.FullName.StartsWith($prefix,[StringComparison]::Ordinal) -and $_.FullName.EndsWith('.png',[StringComparison]::Ordinal)})
  Same-Names @($images | ForEach-Object {$_.FullName}) $expected 'Expected exact 32 browser screenshot entries'
  foreach($entry in $images){
    Require ($entry.Length -ge 8 -and $entry.Length -le 16777216) 'Browser screenshot entry size outside bound'
    $stream=$entry.Open()
    try {
      $header=[byte[]]::new(8)
      $offset=0
      while($offset -lt 8){$read=$stream.Read($header,$offset,8-$offset);Require ($read -gt 0) 'Browser screenshot PNG header incomplete';$offset+=$read}
      Require ([Convert]::ToHexString($header) -ceq '89504E470D0A1A0A') 'Browser screenshot entry lacks PNG signature'
    } finally {$stream.Dispose()}
  }
  return [ordered]@{entry=$parsed.entry;sha256=$parsed.sha256;profiles=$profiles.profiles;routes_per_profile=$profiles.routes_per_profile;route_measurements=$profiles.route_measurements;command_palette_profiles=$profiles.command_palette_profiles;environment_selector_profiles=$profiles.environment_selector_profiles;offline_selected_theme_overrides_device=$profiles.offline_selected_theme_overrides_device;screenshot_png_entries=$images.Count;screenshot_png_headers_checked=$true;scope=$profiles.scope}
}

function Archive-Metrics([string]$Path) {
  $info=Get-Item -LiteralPath $Path
  Require ($info -is [IO.FileInfo] -and ($info.Attributes -band [IO.FileAttributes]::ReparsePoint) -eq 0 -and $info.Length -gt 0 -and $info.Length -le 268435456) 'Local browser archive outside input bounds'
  Add-Type -AssemblyName System.IO.Compression
  $archiveFile=[IO.File]::OpenRead($info.FullName)
  $zip=$null
  try {
    $zip=[IO.Compression.ZipArchive]::new($archiveFile,[IO.Compression.ZipArchiveMode]::Read,$false)
    Require ($zip.Entries.Count -le 1000) 'ZIP inventory exceeds bound'
    $guest=Check-Report (Zip-Report $zip '.tmp/browser-results.json') 'guest' 12
    $normalEN=Check-Report (Zip-Report $zip '.tmp/browser-results-normal-en.json') 'normal-en' 1
    $normalFA=Check-Report (Zip-Report $zip '.tmp/browser-results-normal-fa.json') 'normal-fa' 1
    $worker=Worker-Metrics $guest.tests
    $httpFailures=Public-HttpFailures $guest.tests
    $browser=if(-not $diagnostic){Browser-Metrics $zip}else{$null}
    $load=if(-not $diagnostic){Load-Metrics $zip}else{$null}
    return @{guest=$guest.summary;normalEN=$normalEN.summary;normalFA=$normalFA.summary;worker=$worker;httpFailures=$httpFailures;browser=$browser;load=$load}
  } finally {if($null -ne $zip){$zip.Dispose()};$archiveFile.Dispose()}
}

if($PSCmdlet.ParameterSetName -ceq 'ArchiveOnly') {
  try {
    $metrics=Archive-Metrics ([IO.Path]::GetFullPath($ValidateArchive))
    [ordered]@{status='local_archive_validated';version=$Version;browser_cases=($metrics.guest.cases+$metrics.normalEN.cases+$metrics.normalFA.cases);passed=($metrics.guest.passed+$metrics.normalEN.passed+$metrics.normalFA.passed);browser_measurements=$metrics.browser;bounded_load_read=$metrics.load;scope='Local archive structure validation only. No fetch, CI identity, browser execution or release acceptance is proved.'} | ConvertTo-Json -Depth 12
    return
  } catch {throw 'Local browser archive validation failed.'}
}

Require ($RunId -gt 0 -and $downloadDirectory.StartsWith($destinationRoot,[StringComparison]::OrdinalIgnoreCase) -and $downloadDirectory.Length -gt $destinationRoot.Length) 'Artifact scratch directory must be a new child of the selected scratch root'
Require (-not (Test-Path -LiteralPath $downloadDirectory) -and -not (Test-Path -LiteralPath $receiptPath)) 'Refusing existing artifact directory or receipt'
try {
  $credentialLines=@('protocol=https','host=github.com','') | git -C $repoRoot credential fill 2>$null
  Require ($LASTEXITCODE -eq 0) 'Existing artifact credential unavailable'
  foreach($line in $credentialLines){if($line -match '^password=(.*)$'){$token=$Matches[1]}}
  Require (-not [string]::IsNullOrWhiteSpace($token)) 'Existing artifact credential missing'
  $credentialLines=$null; $line=$null; $Matches=@{}
  $handler=[Net.Http.HttpClientHandler]::new()
  $handler.AllowAutoRedirect=$false
  $client=[Net.Http.HttpClient]::new($handler)
  $client.Timeout=[TimeSpan]::FromSeconds(60)
  $client.DefaultRequestHeaders.UserAgent.ParseAdd('Stackplore-release-review')
  $run=Api-Json "actions/runs/$RunId"
  Require ($run.id -eq $RunId -and $run.head_sha -ceq $Revision -and $run.head_branch -ceq 'main' -and $run.path -ceq '.github/workflows/ci.yml' -and $run.status -ceq 'completed' -and $run.conclusion -ceq $ExpectedConclusion) 'Final CI identity or conclusion does not match the authorized source'
  $jobs=Api-Json "actions/runs/$RunId/jobs?per_page=100"
  Check-CIJobs $jobs $Version $diagnostic
  $listing=Api-Json "actions/runs/$RunId/artifacts?per_page=100"
  Require ($listing.total_count -ge 1 -and $listing.total_count -le 100 -and @($listing.artifacts).Count -eq $listing.total_count) 'Artifact list exceeds one bounded complete page'
  $artifacts=@($listing.artifacts | Where-Object {$_.name -ceq 'browser-evidence'})
  Require ($artifacts.Count -eq 1) 'Expected one unique browser-evidence artifact'
  $artifact=$artifacts[0]
  Require ($artifact.id -gt 0 -and $artifact.expired -is [bool] -and -not $artifact.expired -and $artifact.size_in_bytes -gt 0 -and $artifact.size_in_bytes -le 268435456 -and $artifact.digest -cmatch '^sha256:[a-f0-9]{64}$' -and $artifact.workflow_run.id -eq $RunId -and $artifact.workflow_run.head_sha -ceq $Revision) 'Browser artifact identity, digest, source or size invalid'
  $null=[IO.Directory]::CreateDirectory($downloadDirectory)
  $request=[Net.Http.HttpRequestMessage]::new([Net.Http.HttpMethod]::Get,"https://api.github.com/repos/$repository/actions/artifacts/$($artifact.id)/zip")
  $request.Headers.Authorization=[Net.Http.Headers.AuthenticationHeaderValue]::new('Bearer',$token)
  $response=$null
  try {
    $response=$client.SendAsync($request,[Net.Http.HttpCompletionOption]::ResponseHeadersRead).GetAwaiter().GetResult()
    Require ([int]$response.StatusCode -eq 302) 'Browser artifact redirect unavailable'
    $location=$response.Headers.Location
    Require ($location.IsAbsoluteUri -and $location.Scheme -ceq 'https' -and $location.Port -eq 443 -and [string]::IsNullOrEmpty($location.UserInfo) -and ($location.Host.EndsWith('.blob.core.windows.net',[StringComparison]::OrdinalIgnoreCase) -or $location.Host.EndsWith('.actions.githubusercontent.com',[StringComparison]::OrdinalIgnoreCase))) 'Unexpected artifact transfer host'
  } finally {if($null -ne $response){$response.Dispose()};$request.Dispose()}
  $request=[Net.Http.HttpRequestMessage]::new([Net.Http.HttpMethod]::Get,$location)
  $response=$null; $stream=$null; $file=$null
  $deadline=[Threading.CancellationTokenSource]::new([TimeSpan]::FromSeconds(60))
  try {
    $response=$client.SendAsync($request,[Net.Http.HttpCompletionOption]::ResponseHeadersRead,$deadline.Token).GetAwaiter().GetResult()
    Require ([int]$response.StatusCode -eq 200 -and ($null -eq $response.Content.Headers.ContentLength -or $response.Content.Headers.ContentLength -le 268435456)) 'Browser artifact transfer unavailable or too large'
    $stream=$response.Content.ReadAsStreamAsync().GetAwaiter().GetResult()
    $file=[IO.File]::Open($zipPath,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
    $buffer=[byte[]]::new(65536)
    [long]$length=0
    while($true){$count=$stream.ReadAsync($buffer,0,$buffer.Length,$deadline.Token).GetAwaiter().GetResult();if($count -eq 0){break};$length+=$count;Require ($length -le 268435456) 'Artifact exceeds transfer bound';$file.Write($buffer,0,$count)}
  } finally {if($null -ne $file){$file.Dispose()};if($null -ne $stream){$stream.Dispose()};if($null -ne $response){$response.Dispose()};$request.Dispose();$deadline.Dispose();$location=$null}
  $digest=Digest $zipPath
  Require ((Get-Item -LiteralPath $zipPath).Length -eq $artifact.size_in_bytes -and "sha256:$digest" -ceq $artifact.digest) 'Actual artifact ZIP differs from API size or digest'
  $metrics=Archive-Metrics $zipPath
  $scope=if($diagnostic){'Read-only failed-CI artifact diagnostics with normal TLS. Selected bounded JSON reports parsed without extraction or execution; screenshot and workload acceptance checks are not run. Normal-mode payloads and credentials are not returned. This diagnostic receipt does not imply acceptance.'}else{'Read-only CI artifact retrieval with normal TLS. Exactly 32 current-version PNG signatures and selected bounded JSON reports checked without extraction or execution. Normal-mode payloads and credentials are not returned. Disposable fixture observations do not prove customer deployment or production capacity.'}
  $receipt=[ordered]@{schema_version=2;status=$(if($diagnostic){'diagnostic'}else{'passed'});version=$Version;observed_at_utc=[DateTime]::UtcNow.ToString('o');revision=$Revision;ci_run=$RunId;ci_conclusion=$run.conclusion;artifact_id=$artifact.id;artifact_name=$artifact.name;artifact_bytes=$artifact.size_in_bytes;artifact_sha256=$digest;zip_path=$zipPath;selected_reports=@($metrics.guest,$metrics.normalEN,$metrics.normalFA);total_browser_cases=($metrics.guest.cases+$metrics.normalEN.cases+$metrics.normalFA.cases);passed_browser_cases=($metrics.guest.passed+$metrics.normalEN.passed+$metrics.normalFA.passed);public_worker_control_read=$metrics.worker;public_http_statuses=$metrics.httpFailures;browser_measurements=$metrics.browser;bounded_load_read=$metrics.load;scope=$scope}
  $output=[IO.File]::Open($receiptPath,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
  try {$bytes=[Text.UTF8Encoding]::new($false).GetBytes(($receipt | ConvertTo-Json -Depth 12)+"`n");$output.Write($bytes,0,$bytes.Length)} finally {$output.Dispose()}
  [ordered]@{status=$receipt.status;ci_conclusion=$run.conclusion;ci_run=$RunId;revision=$Revision;artifact_id=$artifact.id;artifact_sha256=$digest;browser_cases=$receipt.total_browser_cases;passed=$receipt.passed_browser_cases;worker_attempts=@($metrics.worker.attempts).Count;receipt=$receiptPath} | ConvertTo-Json -Compress
} catch {throw 'Browser evidence retrieval or validation failed.'} finally {$token=$null;$credentialLines=$null;$line=$null;$Matches=@{};if($null -ne $client){$client.Dispose()};if($null -ne $handler){$handler.Dispose()}}
