#Requires -Version 7.4
[CmdletBinding()]
param()
$ErrorActionPreference='Stop'
$repoRoot=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$consumer=Join-Path $PSScriptRoot 'verify-browser-evidence.ps1'
$version='2.0.0-alpha.0'
$proofRoot=Join-Path $repoRoot '.tmp/v2-browser-consumer-proof'
$workRoot=Join-Path $proofRoot ([Guid]::NewGuid().ToString('N'))
$null=[IO.Directory]::CreateDirectory($workRoot)
$utf8=[Text.UTF8Encoding]::new($false,$true)
$measurementEntry="docs/evidence/$version/browser-measurements.json"
$screenPrefix="docs/evidence/$version/screenshots/"
$marker='synthetic-untrusted-text-must-stay-private'

function Assert([bool]$Condition,[string]$Reason) {if(-not $Condition){throw $Reason}}
function New-Report([int]$Count) {
  $cases=@()
  for($i=0;$i -lt $Count;$i++){
    $cases+=@{expectedStatus='passed';status='expected';results=@(@{status='passed';retry=0;errors=@();attachments=@()})}
  }
  return @{suites=@(@{specs=@(@{tests=$cases});suites=@()});stats=@{expected=$Count;unexpected=0;flaky=0;skipped=0};errors=@()}
}
function New-Fixture {
  $guest=New-Report 12
  $worker=@{deadlineMs=15000;maximumAttempts=4;attempts=@(@{attempt=1;status=200;mime='application/javascript';bytes=2048;cacheMarkers=1})}
  $guest.suites[0].specs[0].tests[0].results[0].attachments=@(@{name='public-worker-control-read';contentType='application/json';body=[Convert]::ToBase64String($utf8.GetBytes(($worker | ConvertTo-Json -Depth 12 -Compress)))})
  $paths=@('/brand/stackplore-favicon-64.png','/offline.css','/offline.html','/offline.js')
  $document=[ordered]@{version=$version}
  $images=@()
  $png=[Convert]::FromBase64String('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jFZ0AAAAASUVORK5CYII=')
  foreach($locale in @('en','fa')){foreach($width in @(360,1280)){foreach($theme in @('light','dark')){
    $key="$locale-$width-$theme"
    $measurements=@()
    foreach($route in @('/overview','/resources','/topology','/logs','/events','/alerts','/notifications','/search')){$measurements+=@{route=$route;routeReadyMs=1;reducedMotion=$true;normalMotionDuration='1ms'}}
    $profile=@{version=$version;chromium='synthetic-checker-fixture';measurements=$measurements;commandPalette=@{keyboard=$true;accessibility=$true;viewport=$true;fixedNavigation=$true;focusReturn=$true};httpFailures=@()}
    if($theme -ceq 'light'){$profile.environmentSelector=@{lazyReads=$true;sourceBound=$true;keyboard=$true;focusReturn=$true;accessibility=$true;viewport=$true;environmentReads=2;mobileDrawer=($width -eq 360)}}
    $document[$key]=$profile
    foreach($component in @('overview','topology','command-palette','about')){$images+=@{Name="$screenPrefix$component-$key.png";Bytes=$png}}
  }}}
  $document.offline=@{keyboardRetry=$true;privateContentAbsent=$true;publicPaths=$paths;selectedThemeOverridesDevice=@('light','dark')}
  $document.installability=@{chromiumProtocolChecked=$true;regularProfile=$true;errors=@();httpFailures=@()}
  $document.update=@{explicitAcceptance=$true;publicPaths=$paths}
  $document.concurrentReads=@{requests=80;parallel=8;rounds=10;minimumRoundSpacingMs=600;setupRateBucketRefillMs=2500;setupNavigationRefills=4;setupNavigationRefillMs=2500;initialRateBucketRefillMs=2500;setupExcludedFromReadTiming=$true;successfulResponses=80;envelopesValidated=$true;elapsedMs=6000;medianMs=1;p95Ms=1}
  return @{Documents=[ordered]@{'.tmp/browser-results.json'=$guest;'.tmp/browser-results-normal-en.json'=(New-Report 1);'.tmp/browser-results-normal-fa.json'=(New-Report 1);$measurementEntry=$document};Images=$images;Extras=@()}
}
function Write-Archive($Fixture,[string]$Path) {
  $file=[IO.File]::Open($Path,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
  $archive=$null
  try {
    $archive=[IO.Compression.ZipArchive]::new($file,[IO.Compression.ZipArchiveMode]::Create,$false)
    $entries=@()
    foreach($key in $Fixture.Documents.Keys){$entries+=@{Name=$key;Bytes=$utf8.GetBytes(($Fixture.Documents[$key] | ConvertTo-Json -Depth 30 -Compress))}}
    $entries+=@($Fixture.Images)+@($Fixture.Extras)
    foreach($item in $entries){
      $entry=$archive.CreateEntry($item.Name,[IO.Compression.CompressionLevel]::Fastest)
      $stream=$entry.Open()
      try {$stream.Write($item.Bytes,0,$item.Bytes.Length)} finally {$stream.Dispose()}
    }
  } finally {if($null -ne $archive){$archive.Dispose()};$file.Dispose()}
}
function Invoke-Consumer([string]$Path,[string]$RequestedVersion) {
  $start=[Diagnostics.ProcessStartInfo]::new()
  $start.FileName=(Get-Process -Id $PID).Path
  $start.UseShellExecute=$false
  $start.CreateNoWindow=$true
  $start.RedirectStandardOutput=$true
  $start.RedirectStandardError=$true
  foreach($argument in @('-NoProfile','-NonInteractive','-File',$consumer,'-Version',$RequestedVersion,'-ValidateArchive',$Path)){$start.ArgumentList.Add($argument)}
  $process=[Diagnostics.Process]::new()
  $process.StartInfo=$start
  try {
    Assert ($process.Start()) 'Consumer process did not start'
    $stdoutTask=$process.StandardOutput.ReadToEndAsync()
    $stderrTask=$process.StandardError.ReadToEndAsync()
    if(-not $process.WaitForExit(30000)){$process.Kill($true);throw 'Consumer process exceeded 30-second bound'}
    return @{ExitCode=$process.ExitCode;Stdout=$stdoutTask.GetAwaiter().GetResult();Stderr=$stderrTask.GetAwaiter().GetResult()}
  } finally {$process.Dispose()}
}

$cases=@(
  @{Name='valid-32-images-14-cases-80-reads';Expected=$true;Mutate={param($fixture)}},
  @{Name='missing-about-image';Mutate={param($fixture) $fixture.Images=@($fixture.Images | Where-Object {$_.Name -cne ($screenPrefix+'about-fa-360-dark.png')})}},
  @{Name='unexpected-image';Mutate={param($fixture) $fixture.Images+=@{Name=$screenPrefix+'unexpected.png';Bytes=$fixture.Images[0].Bytes}}},
  @{Name='duplicate-image';Mutate={param($fixture) $fixture.Images+=@{Name=$fixture.Images[0].Name;Bytes=$fixture.Images[0].Bytes}}},
  @{Name='invalid-png-signature';Mutate={param($fixture) $fixture.Images[0].Bytes=$utf8.GetBytes('not-a-png-header')}},
  @{Name='duplicate-report';Mutate={param($fixture) $fixture.Extras+=@{Name='.tmp/browser-results.json';Bytes=$utf8.GetBytes('{}')}}},
  @{Name='old-offline-svg';Mutate={param($fixture) $fixture.Documents[$measurementEntry].offline.publicPaths[0]='/brand/stackplore-favicon.svg'}},
  @{Name='offline-opposite-device-theme-missing';Mutate={param($fixture) $fixture.Documents[$measurementEntry].offline.Remove('selectedThemeOverridesDevice')}},
  @{Name='private-update-cache';Mutate={param($fixture) $fixture.Documents[$measurementEntry].update.publicPaths[0]='/api/v1/session'}},
  @{Name='wrong-profile-version';Mutate={param($fixture) $fixture.Documents[$measurementEntry]['fa-360-dark'].version='1.0.3'}},
  @{Name='missing-profile';Mutate={param($fixture) $fixture.Documents[$measurementEntry].Remove('fa-1280-dark')}},
  @{Name='missing-route';Mutate={param($fixture) $fixture.Documents[$measurementEntry]['en-1280-dark'].measurements=@($fixture.Documents[$measurementEntry]['en-1280-dark'].measurements | Where-Object {$_.route -cne '/topology'})}},
  @{Name='palette-keyboard-missing';Mutate={param($fixture) $fixture.Documents[$measurementEntry]['fa-360-dark'].commandPalette.Remove('keyboard')}},
  @{Name='environment-read-budget';Mutate={param($fixture) $fixture.Documents[$measurementEntry]['fa-360-light'].environmentSelector.environmentReads=3}},
  @{Name='unexecuted-dark-environment';Mutate={param($fixture) $fixture.Documents[$measurementEntry]['fa-360-dark'].environmentSelector=$fixture.Documents[$measurementEntry]['fa-360-light'].environmentSelector}},
  @{Name='read-count-79';Mutate={param($fixture) $fixture.Documents[$measurementEntry].concurrentReads.requests=79}},
  @{Name='string-read-count';Mutate={param($fixture) $fixture.Documents[$measurementEntry].concurrentReads.requests='80'}},
  @{Name='load-missing';Mutate={param($fixture) $fixture.Documents[$measurementEntry].Remove('concurrentReads')}},
  @{Name='round-spacing-changed';Mutate={param($fixture) $fixture.Documents[$measurementEntry].concurrentReads.minimumRoundSpacingMs=0}},
  @{Name='setup-refill-changed';Mutate={param($fixture) $fixture.Documents[$measurementEntry].concurrentReads.setupNavigationRefillMs=500}},
  @{Name='read-response-failed';Mutate={param($fixture) $fixture.Documents[$measurementEntry].concurrentReads.successfulResponses=79}},
  @{Name='retry-used';Mutate={param($fixture) $fixture.Documents['.tmp/browser-results-normal-en.json'].suites[0].specs[0].tests[0].results[0].retry=1}},
  @{Name='two-attempt-pass';Mutate={param($fixture) $case=$fixture.Documents['.tmp/browser-results-normal-fa.json'].suites[0].specs[0].tests[0];$case.results+=@{status='passed';retry=1;errors=@();attachments=@()}}},
  @{Name='flaky-report';Mutate={param($fixture) $fixture.Documents['.tmp/browser-results.json'].stats.flaky=1}},
  @{Name='skipped-case';Mutate={param($fixture) $fixture.Documents['.tmp/browser-results.json'].suites[0].specs[0].tests[1].status='skipped'}},
  @{Name='report-error-not-returned';Mutate={param($fixture) $fixture.Documents['.tmp/browser-results-normal-en.json'].errors=@(@{message=$marker})}},
  @{Name='worker-proof-missing';Mutate={param($fixture) $fixture.Documents['.tmp/browser-results.json'].suites[0].specs[0].tests[0].results[0].attachments=@()}},
  @{Name='wrong-browser-case-count';Mutate={param($fixture) $fixture.Documents['.tmp/browser-results.json'].suites[0].specs[0].tests=@($fixture.Documents['.tmp/browser-results.json'].suites[0].specs[0].tests[0..10])}},
  @{Name='noncanonical-request-version';RequestedVersion='2.00.0-alpha.0';ParameterError=$true;Mutate={param($fixture)}}
)
$passed=@()
foreach($case in $cases){
  $fixture=New-Fixture
  $null=& $case.Mutate $fixture
  $path=Join-Path $workRoot ($case.Name+'.zip')
  Write-Archive $fixture $path
  $requestedVersion=if($case.ContainsKey('RequestedVersion')){$case.RequestedVersion}else{$version}
  $actual=Invoke-Consumer $path $requestedVersion
  Assert (-not ($actual.Stdout+$actual.Stderr).Contains($marker,[StringComparison]::Ordinal)) ('Untrusted report text escaped: '+$case.Name)
  if($case.Expected){
    Assert ($actual.ExitCode -eq 0 -and $actual.Stderr.Length -eq 0) ('Expected valid archive: '+$case.Name)
    $summary=$actual.Stdout | ConvertFrom-Json
    Assert ($summary.status -ceq 'local_archive_validated' -and $summary.version -ceq $version -and $summary.browser_cases -eq 14 -and $summary.passed -eq 14 -and $summary.browser_measurements.screenshot_png_entries -eq 32 -and $summary.browser_measurements.route_measurements -eq 64 -and @($summary.browser_measurements.environment_selector_profiles).Count -eq 4 -and $summary.bounded_load_read.requests -eq 80 -and $summary.bounded_load_read.successful_responses -eq 80 -and $summary.scope -ceq 'Local archive structure validation only. No fetch, CI identity, browser execution or release acceptance is proved.') 'Local fixture output overstated or omitted structural proof'
  } else {
    Assert ($actual.ExitCode -ne 0 -and $actual.Stdout.Length -eq 0) ('Expected rejected archive: '+$case.Name)
    if(-not $case.ParameterError){Assert ($actual.Stderr.Contains('Local browser archive validation failed.',[StringComparison]::Ordinal)) ('Failure did not use fixed safe error: '+$case.Name)}
  }
  $passed+=@{case=$case.Name;status='passed';expected_acceptance=[bool]$case.Expected}
}
$result=[ordered]@{schema_version=1;status='passed';observed_at_utc=[DateTime]::UtcNow.ToString('o');checks=$passed.Count;valid_archives=1;invalid_archives=($passed.Count-1);consumer_sha256=(Get-FileHash -LiteralPath $consumer -Algorithm SHA256).Hash.ToLowerInvariant();test_sha256=(Get-FileHash -LiteralPath $PSCommandPath -Algorithm SHA256).Hash.ToLowerInvariant();cases=$passed;scope='Synthetic local ZIP regression only. No network, credential read, browser execution, CI artifact, customer deployment or release acceptance is proved.'}
$resultPath=Join-Path $workRoot 'summary.json'
[IO.File]::WriteAllText($resultPath,($result | ConvertTo-Json -Depth 8)+"`n",$utf8)
[ordered]@{status=$result.status;checks=$result.checks;valid_archives=$result.valid_archives;invalid_archives=$result.invalid_archives;receipt=$resultPath;scope=$result.scope} | ConvertTo-Json -Compress
