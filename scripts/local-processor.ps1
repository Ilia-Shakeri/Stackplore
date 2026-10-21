#Requires -Version 7.4
[CmdletBinding()]
param(
  [ValidateSet('Check','Start','Stop')][string]$Action='Check',
  [string]$Archive,
  [string]$RuntimeDirectory,
  [string]$Model,
  [string]$ModelSHA256,
  [string]$ReceiptPath,
  [ValidateRange(2,4)][int]$Threads=2,
  [ValidateRange(10,120)][int]$StartupSeconds=120
)

Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$script:ProcessorArchiveSHA256='aef737d26bffd08ceeb7be75f045bdef6d1ecc3187e0f4a1a3b0d878a03d2550'
$script:ProcessorSource='412a363c655b479279d3050d0d1c67a5733697b4'
$script:ProcessorBuild='b11729-412a363c6'
$script:ProcessorPort=58102

function Assert-Processor([bool]$Condition,[string]$Code) {
  if(-not $Condition){throw [InvalidOperationException]::new($Code)}
}

function Get-ProcessorFullPath([string]$Path) {
  Assert-Processor (-not [string]::IsNullOrWhiteSpace($Path) -and [IO.Path]::IsPathFullyQualified($Path) -and $Path -notmatch '[\x00-\x1f"*?]') 'processor.path_invalid'
  $full=[IO.Path]::GetFullPath($Path)
  Assert-Processor ($full -cmatch '\A[A-Za-z]:[\\/]' -and $full.Substring(2) -notmatch ':') 'processor.local_drive_required'
  return $full
}

function Get-ProcessorItem([string]$Path,[bool]$Directory,[long]$Minimum=1,[long]$Maximum=8589934592) {
  $full=Get-ProcessorFullPath $Path
  $item=Get-Item -LiteralPath $full -Force -ErrorAction Stop
  Assert-Processor ($item.PSIsContainer -eq $Directory) 'processor.path_kind'
  if(-not $Directory){Assert-Processor ($item.Length -ge $Minimum -and $item.Length -le $Maximum) 'processor.file_size'}
  $ancestor=$item
  while($null -ne $ancestor){
    Assert-Processor (($ancestor.Attributes -band [IO.FileAttributes]::ReparsePoint) -eq 0) 'processor.link_refused'
    $ancestor=if($ancestor -is [IO.DirectoryInfo]){$ancestor.Parent}else{$ancestor.Directory}
  }
  return $item
}

function Get-ProcessorHash([IO.Stream]$Stream) {
  return [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData($Stream)).ToLowerInvariant()
}

function Assert-ProcessorStreamSize([IO.Stream]$Stream,[long]$Minimum,[long]$Maximum) {
  Assert-Processor ($Stream.CanSeek -and $Stream.Length -ge $Minimum -and $Stream.Length -le $Maximum) 'processor.open_file_size'
}

function Test-ProcessorDigest([string]$Digest) {return $Digest -cmatch '\A[a-f0-9]{64}\z'}

function Test-ProcessorArchiveName([string]$Name) {
  return $Name -cmatch '\A[A-Za-z0-9][A-Za-z0-9_.-]{0,127}\z' -and $Name -cnotmatch '\A(?i:con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|\z)' -and -not $Name.EndsWith('.')
}

function Test-ProcessorModelHeader([byte[]]$Header) {
  return $Header.Length -eq 24 -and [Text.Encoding]::ASCII.GetString($Header,0,4) -ceq 'GGUF' -and [BitConverter]::ToUInt32($Header,4) -eq 3 -and [BitConverter]::ToUInt64($Header,8) -ge 1 -and [BitConverter]::ToUInt64($Header,8) -le 100000 -and [BitConverter]::ToUInt64($Header,16) -ge 1 -and [BitConverter]::ToUInt64($Header,16) -le 65536
}

# Hold read-only handles through startup so verified files cannot be replaced during launch.
function Open-ProcessorInputs([string]$ArchivePath,[string]$Directory,[string]$ModelPath,[string]$ExpectedModelSHA256) {
  Assert-Processor (Test-ProcessorDigest $ExpectedModelSHA256) 'processor.model_digest_invalid'
  $handles=[Collections.Generic.List[IO.Stream]]::new()
  $zip=$null
  try {
    $archiveItem=Get-ProcessorItem $ArchivePath $false 1048576 268435456
    $runtimeItem=Get-ProcessorItem $Directory $true
    $modelItem=Get-ProcessorItem $ModelPath $false 16777216 8589934592
    Assert-Processor ($modelItem.Extension -ceq '.gguf') 'processor.model_extension'
    $archiveStream=[IO.File]::Open($archiveItem.FullName,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::Read)
    $handles.Add($archiveStream)
    Assert-ProcessorStreamSize $archiveStream 1048576 268435456
    Assert-Processor ((Get-ProcessorHash $archiveStream) -ceq $script:ProcessorArchiveSHA256) 'processor.archive_digest'
    $archiveStream.Position=0
    $zip=[IO.Compression.ZipArchive]::new($archiveStream,[IO.Compression.ZipArchiveMode]::Read,$true)
    Assert-Processor ($zip.Entries.Count -eq 51) 'processor.archive_inventory'
    $actual=@(Get-ChildItem -LiteralPath $runtimeItem.FullName -Force)
    Assert-Processor ($actual.Count -eq 51) 'processor.install_inventory'
    $seen=[Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
    $inventory=[Collections.Generic.List[string]]::new()
    $executable=$null
    foreach($entry in $zip.Entries){
      $type=($entry.ExternalAttributes -shr 16) -band 61440
      Assert-Processor ((Test-ProcessorArchiveName $entry.FullName) -and $seen.Add($entry.FullName) -and $entry.Length -ge 1 -and $entry.Length -le 33554432 -and $type -in @(0,32768) -and ($entry.ExternalAttributes -band 1040) -eq 0) 'processor.archive_entry'
      $matches=@($actual | Where-Object {$_.Name -ceq $entry.FullName})
      Assert-Processor ($matches.Count -eq 1) 'processor.install_inventory'
      $installed=Get-ProcessorItem $matches[0].FullName $false $entry.Length $entry.Length
      $installedStream=[IO.File]::Open($installed.FullName,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::Read)
      $handles.Add($installedStream)
      Assert-ProcessorStreamSize $installedStream $entry.Length $entry.Length
      $installedDigest=Get-ProcessorHash $installedStream
      $entryStream=$entry.Open()
      try {$entryDigest=Get-ProcessorHash $entryStream} finally {$entryStream.Dispose()}
      Assert-Processor ($installedDigest -ceq $entryDigest) 'processor.install_digest'
      $inventory.Add($entry.FullName+"`0"+$entry.Length+"`0"+$installedDigest)
      if($entry.FullName -ceq 'llama-server.exe'){$executable=@{path=$installed.FullName;sha256=$installedDigest}}
    }
    Assert-Processor ($null -ne $executable -and $seen.Contains('llama-server-impl.dll')) 'processor.server_missing'
    $modelStream=[IO.File]::Open($modelItem.FullName,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::Read)
    $handles.Add($modelStream)
    Assert-ProcessorStreamSize $modelStream 16777216 8589934592
    $header=[byte[]]::new(24)
    Assert-Processor ($modelStream.Read($header,0,24) -eq 24 -and (Test-ProcessorModelHeader $header)) 'processor.model_header'
    $modelStream.Position=0
    Assert-Processor ((Get-ProcessorHash $modelStream) -ceq $ExpectedModelSHA256) 'processor.model_digest'
    $inventoryBytes=[Text.Encoding]::UTF8.GetBytes((@($inventory | Sort-Object -CaseSensitive) -join "`n"))
    $inventoryDigest=[Convert]::ToHexString([Security.Cryptography.SHA256]::HashData($inventoryBytes)).ToLowerInvariant()
    return @{handles=$handles;zip=$zip;runtime=$runtimeItem.FullName;executable=$executable;model=$modelItem.FullName;model_bytes=$modelStream.Length;inventory_sha256=$inventoryDigest}
  } catch {
    if($null -ne $zip){$zip.Dispose()}
    foreach($handle in $handles){$handle.Dispose()}
    throw
  }
}

function Close-ProcessorInputs($Inputs) {
  if($null -eq $Inputs){return}
  $Inputs.zip.Dispose()
  foreach($handle in $Inputs.handles){$handle.Dispose()}
}

function Get-ProcessorArguments([string]$ModelPath,[string]$Digest,[int]$ThreadCount) {
  Assert-Processor ((Test-ProcessorDigest $Digest) -and $ThreadCount -ge 2 -and $ThreadCount -le 4 -and -not [string]::IsNullOrWhiteSpace($ModelPath) -and $ModelPath -notmatch '[\x00-\x1f"]') 'processor.arguments_invalid'
  return @('--model',$ModelPath,'--alias',"sha256-$Digest",'--host','127.0.0.1','--port','58102','--cors-origins','http://127.0.0.1:58102','--parallel','1','--ctx-size','20480','--threads',"$ThreadCount",'--threads-batch',"$ThreadCount",'--threads-http','2','--device','none','--gpu-layers','0','--fit','off','--cache-type-k','q8_0','--cache-type-v','q8_0','--flash-attn','on','--batch-size','512','--ubatch-size','256','--predict','4096','--timeout','30','--offline','--no-webui','--no-agent','--no-ui-mcp-proxy','--no-slots','--no-models-autoload','--no-mmproj','--no-cache-prompt','--cache-ram','0','--no-cache-idle-slots','--no-context-shift','--ctx-checkpoints','0','--no-cont-batching','--reasoning','off','--log-disable')
}

function ConvertTo-ProcessorCommandLine([string[]]$Arguments) {
  foreach($argument in $Arguments){Assert-Processor ($argument -notmatch '[\x00-\x1f"]' -and -not $argument.EndsWith('\')) 'processor.argument_quoting'}
  return (@($Arguments | ForEach-Object {'"'+$_+'"'}) -join ' ')
}

function Get-ProcessorEnvironment([Collections.IDictionary]$Inherited) {
  $overrides=@{}
  foreach($key in $Inherited.Keys){
    if([string]$key -match '(?i)\A(?:LLAMA|GGML|MTMD|HF|HUGGINGFACE|CURL|SSL_CERT|HTTP_PROXY|HTTPS_PROXY|ALL_PROXY|NO_PROXY)' -or [string]$key -match '(?i)(?:TOKEN|SECRET|PASSWORD|CREDENTIAL|KEY|AUTH|COOKIE|SESSION)'){$overrides[[string]$key]=$null}
  }
  return $overrides
}

function Test-ProcessorOwner($Record,$Process) {
  try {
    return $Record.schema -eq 1 -and $Record.kind -ceq 'stackplore.operator.local-processor' -and $Record.source -ceq $script:ProcessorSource -and $Record.archive_sha256 -ceq $script:ProcessorArchiveSHA256 -and $Record.pid -gt 0 -and $Record.start_utc_ticks -gt 0 -and $Process.Id -eq $Record.pid -and $Process.StartTime.ToUniversalTime().Ticks -eq $Record.start_utc_ticks -and [string]::Equals($Process.Path,$Record.executable,[StringComparison]::OrdinalIgnoreCase) -and -not $Process.HasExited
  } catch {return $false}
}

function Get-ProcessorListeners {
  return @(Get-NetTCPConnection -State Listen -ErrorAction Stop | Where-Object {$_.LocalPort -eq $script:ProcessorPort})
}

function Assert-ProcessorListener($Process) {
  $listeners=@(Get-ProcessorListeners)
  if($listeners.Count -eq 0){return $false}
  Assert-Processor ($listeners.Count -eq 1 -and $listeners[0].LocalAddress -ceq '127.0.0.1' -and $listeners[0].OwningProcess -eq $Process.Id) 'processor.foreign_listener'
  return $true
}

function Read-ProcessorBytes([IO.Stream]$Stream,[Threading.CancellationToken]$Cancellation) {
  $contents=[IO.MemoryStream]::new()
  try {
    $buffer=[byte[]]::new(8192)
    while(($length=$Stream.ReadAsync($buffer,0,$buffer.Length,$Cancellation).GetAwaiter().GetResult()) -gt 0){
      Assert-Processor ($contents.Length+$length -le 1048576) 'processor.probe_size'
      $contents.Write($buffer,0,$length)
    }
    return ,$contents.ToArray()
  } finally {$contents.Dispose()}
}

function Read-ProcessorJSON([string]$Path) {
  Assert-Processor ($Path -cin @('/health','/props','/v1/models')) 'processor.probe_path'
  $handler=[Net.Http.HttpClientHandler]::new()
  $handler.UseProxy=$false
  $handler.AllowAutoRedirect=$false
  $handler.AutomaticDecompression=[Net.DecompressionMethods]::None
  $client=[Net.Http.HttpClient]::new($handler)
  $client.Timeout=[TimeSpan]::FromSeconds(2)
  $response=$null
  $stream=$null
  $cancel=[Threading.CancellationTokenSource]::new(2000)
  try {
    $response=$client.GetAsync('http://127.0.0.1:58102'+$Path,[Net.Http.HttpCompletionOption]::ResponseHeadersRead,$cancel.Token).GetAwaiter().GetResult()
    if([int]$response.StatusCode -eq 503){return $null}
    Assert-Processor ([int]$response.StatusCode -eq 200 -and $response.Content.Headers.ContentLength -le 1048576 -and $response.Content.Headers.ContentEncoding.Count -eq 0) 'processor.probe_response'
    $stream=$response.Content.ReadAsStream($cancel.Token)
    $body=Read-ProcessorBytes $stream $cancel.Token
    $json=[Text.UTF8Encoding]::new($false,$true).GetString($body)
    return ConvertFrom-Json -InputObject $json -AsHashtable -Depth 32 -ErrorAction Stop
  } finally {
    if($null -ne $stream){$stream.Dispose()}
    if($null -ne $response){$response.Dispose()}
    $cancel.Dispose();$client.Dispose();$handler.Dispose()
  }
}

function Write-ProcessorReceipt([IO.FileStream]$Stream,$Record) {
  $bytes=[Text.UTF8Encoding]::new($false).GetBytes(($Record | ConvertTo-Json -Depth 8))
  Assert-Processor ($bytes.Length -le 16384) 'processor.receipt_size'
  $Stream.Position=0
  $Stream.SetLength(0)
  $Stream.Write($bytes,0,$bytes.Length)
  $Stream.Flush($true)
}

function Open-ProcessorReceipt([string]$Path,[bool]$Create) {
  Assert-Processor (-not [string]::IsNullOrWhiteSpace($Path) -and [IO.Path]::IsPathFullyQualified($Path) -and $Path -notmatch '[\x00-\x1f"*?]' -and [IO.Path]::GetExtension($Path) -ceq '.json') 'processor.receipt_path'
  $full=Get-ProcessorFullPath $Path
  $null=Get-ProcessorItem ([IO.Path]::GetDirectoryName($full)) $true
  if(-not $Create){$null=Get-ProcessorItem $full $false 1 16384}
  $mode=if($Create){[IO.FileMode]::CreateNew}else{[IO.FileMode]::Open}
  $stream=[IO.File]::Open($full,$mode,[IO.FileAccess]::ReadWrite,[IO.FileShare]::None)
  try {Assert-ProcessorStreamSize $stream $(if($Create){0}else{1}) 16384;return $stream} catch {$stream.Dispose();throw}
}

function Stop-OwnedProcessor($Record,$Process) {
  Assert-Processor (Test-ProcessorOwner $Record $Process) 'processor.ownership_refused'
  $null=Get-ProcessorItem $Record.executable $false 1 33554432
  $stream=[IO.File]::Open($Record.executable,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::Read)
  try {Assert-ProcessorStreamSize $stream 1 33554432;Assert-Processor ((Get-ProcessorHash $stream) -ceq $Record.executable_sha256) 'processor.ownership_digest'} finally {$stream.Dispose()}
  Assert-Processor (Test-ProcessorOwner $Record $Process) 'processor.ownership_refused'
  $Process.Kill()
  Assert-Processor ($Process.WaitForExit(5000)) 'processor.stop_timeout'
}

function Invoke-LocalProcessor {
  Assert-Processor ($IsWindows) 'processor.windows_required'
  $inputs=$null
  $receipt=$null
  $process=$null
  $record=$null
  $ownsReceipt=$false
  try {
    if($Action -ceq 'Stop'){
      $receipt=Open-ProcessorReceipt $ReceiptPath $false
      $reader=[IO.StreamReader]::new($receipt,[Text.UTF8Encoding]::new($false,$true),$false,4096,$true)
      try {$record=ConvertFrom-Json -InputObject $reader.ReadToEnd() -AsHashtable -Depth 8} finally {$reader.Dispose()}
      $process=Get-Process -Id $record.pid -ErrorAction Stop
      Assert-Processor (Test-ProcessorOwner $record $process) 'processor.ownership_refused'
      $ownsReceipt=$true
      Stop-OwnedProcessor $record $process
      $record.status='stopped';$record.stopped_at=[DateTime]::UtcNow.ToString('o')
      Write-ProcessorReceipt $receipt $record
      return @{status='stopped';owned=$true;pid=$record.pid}
    }
    $inputs=Open-ProcessorInputs $Archive $RuntimeDirectory $Model $ModelSHA256
    $summary=@{status='checked';build=$script:ProcessorBuild;source=$script:ProcessorSource;archive_sha256=$script:ProcessorArchiveSHA256;inventory_sha256=$inputs.inventory_sha256;installed_files=51;model_sha256=$ModelSHA256;model_bytes=$inputs.model_bytes;alias="sha256-$ModelSHA256";context=20480;slots=1;threads=$Threads}
    if($Action -ceq 'Check'){return $summary}
    Assert-Processor (@(Get-ProcessorListeners).Count -eq 0) 'processor.port_in_use'
    $receipt=Open-ProcessorReceipt $ReceiptPath $true
    $ownsReceipt=$true
    $arguments=Get-ProcessorArguments $inputs.model $ModelSHA256 $Threads
    $record=@{schema=1;kind='stackplore.operator.local-processor';status='starting';source=$script:ProcessorSource;build=$script:ProcessorBuild;archive_sha256=$script:ProcessorArchiveSHA256;inventory_sha256=$inputs.inventory_sha256;executable=$inputs.executable.path;executable_sha256=$inputs.executable.sha256;model=$inputs.model;model_sha256=$ModelSHA256;model_bytes=$inputs.model_bytes;pid=0;start_utc_ticks=0;created_at=[DateTime]::UtcNow.ToString('o');host='127.0.0.1';port=$script:ProcessorPort;context=20480;slots=1;threads=$Threads;kv_type='q8_0';cache=$false}
    Write-ProcessorReceipt $receipt $record
    $environment=Get-ProcessorEnvironment ([Environment]::GetEnvironmentVariables())
    $process=Start-Process -FilePath $inputs.executable.path -WorkingDirectory $inputs.runtime -ArgumentList (ConvertTo-ProcessorCommandLine $arguments) -Environment $environment -WindowStyle Hidden -PassThru -ErrorAction Stop
    $record.pid=$process.Id;$record.start_utc_ticks=$process.StartTime.ToUniversalTime().Ticks
    Assert-Processor (Test-ProcessorOwner $record $process) 'processor.start_ownership'
    Write-ProcessorReceipt $receipt $record
    $deadline=[DateTime]::UtcNow.AddSeconds($StartupSeconds)
    $ready=$false
    while([DateTime]::UtcNow -lt $deadline){
      Assert-Processor (Test-ProcessorOwner $record $process) 'processor.start_exited'
      if(Assert-ProcessorListener $process){
        try {$health=Read-ProcessorJSON '/health';$ready=$null -ne $health -and $health.status -ceq 'ok'} catch {$ready=$false}
        if($ready){break}
      }
      Start-Sleep -Milliseconds 250
    }
    Assert-Processor $ready 'processor.start_timeout'
    $null=Assert-ProcessorListener $process
    $properties=Read-ProcessorJSON '/props'
    $models=Read-ProcessorJSON '/v1/models'
    Assert-Processor ($properties.build_info -ceq $script:ProcessorBuild -and $properties.total_slots -eq 1 -and $properties.default_generation_settings.n_ctx -eq 20480 -and $properties.chat_template -is [string] -and $properties.chat_template.Length -gt 0 -and [Text.Encoding]::UTF8.GetByteCount($properties.chat_template) -le 65536 -and $models.data.Count -eq 1 -and $models.data[0].id -ceq "sha256-$ModelSHA256") 'processor.identity_mismatch'
    $templateSHA256=[Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes($properties.chat_template))).ToLowerInvariant()
    Assert-Processor (Test-ProcessorOwner $record $process) 'processor.start_ownership'
    $null=Assert-ProcessorListener $process
    $record.status='ready';$record.ready_at=[DateTime]::UtcNow.ToString('o');$record.template_sha256=$templateSHA256
    Write-ProcessorReceipt $receipt $record
    $summary.status='ready';$summary.pid=$record.pid;$summary.template_sha256=$templateSHA256
    return $summary
  } catch {
    $code='processor.operation_failed'
    if($_.Exception.Message -cmatch '\Aprocessor\.[a-z_]+\z'){$code=$_.Exception.Message}
    if($ownsReceipt -and $null -ne $record -and $null -ne $receipt){
      $record.status='failed';$record.error_code=$code
      if($null -ne $process -and (Test-ProcessorOwner $record $process)){
        try {Stop-OwnedProcessor $record $process;$record.owned_process_stopped=$true} catch {$record.owned_process_stopped=$false}
      }
      try {Write-ProcessorReceipt $receipt $record} catch {}
    }
    throw [InvalidOperationException]::new($code)
  } finally {
    if($null -ne $receipt){$receipt.Dispose()}
    if($null -ne $process){$process.Dispose()}
    Close-ProcessorInputs $inputs
  }
}

if($MyInvocation.InvocationName -ne '.'){
  try {Invoke-LocalProcessor | ConvertTo-Json -Depth 6} catch {Write-Output ('{"status":"failed","error_code":"'+$_.Exception.Message+'"}');exit 1}
}
