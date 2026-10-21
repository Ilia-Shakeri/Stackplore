#Requires -Version 7.4
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'local-processor.ps1')
$script:Checks=0
function Check([bool]$Condition,[string]$Label){if(-not $Condition){throw "processor_test.$Label"};$script:Checks++}
function Reject([scriptblock]$Operation,[string]$Label){$rejected=$false;try{& $Operation | Out-Null}catch{$rejected=$true};Check $rejected $Label}

Check ($Action -ceq 'Check') 'default_action'
Check ($script:ProcessorBuild -ceq 'b11729-412a363c6') 'build_pin'
Check ($script:ProcessorSource -ceq '412a363c655b479279d3050d0d1c67a5733697b4') 'source_pin'
Check ($script:ProcessorArchiveSHA256 -ceq 'aef737d26bffd08ceeb7be75f045bdef6d1ecc3187e0f4a1a3b0d878a03d2550') 'archive_pin'
$digest='a'*64
Check (Test-ProcessorDigest $digest) 'digest_positive'
foreach($bad in @(('A'*64),('a'*63),('a'*65),('g'*64),"$digest`n",'')){Check (-not (Test-ProcessorDigest $bad)) 'digest_negative'}
foreach($name in @('llama-server.exe','ggml-cpu-x64.dll','LICENSE-LLVM-OpenMP')){Check (Test-ProcessorArchiveName $name) 'entry_positive'}
foreach($name in @('../file','a/b','a\b','a:b','file.','CON','aux.dll','lpt1.txt','',('x'+[char]0))){Check (-not (Test-ProcessorArchiveName $name)) 'entry_negative'}

$header=[byte[]]::new(24)
[Text.Encoding]::ASCII.GetBytes('GGUF').CopyTo($header,0)
[BitConverter]::GetBytes([uint32]3).CopyTo($header,4)
[BitConverter]::GetBytes([uint64]1).CopyTo($header,8)
[BitConverter]::GetBytes([uint64]1).CopyTo($header,16)
Check (Test-ProcessorModelHeader $header) 'header_positive'
foreach($offset in @(0,4,8,16)){$bad=[byte[]]$header.Clone();$bad[$offset]=0;Check (-not (Test-ProcessorModelHeader $bad)) 'header_negative'}
Check (-not (Test-ProcessorModelHeader ([byte[]]::new(23)))) 'header_short'
$bad=[byte[]]$header.Clone();[BitConverter]::GetBytes([uint64]100001).CopyTo($bad,8);Check (-not (Test-ProcessorModelHeader $bad)) 'header_tensor_bound'
$bad=[byte[]]$header.Clone();[BitConverter]::GetBytes([uint64]65537).CopyTo($bad,16);Check (-not (Test-ProcessorModelHeader $bad)) 'header_metadata_bound'
Check ((Get-ProcessorFullPath 'D:\operator data\model.gguf') -ceq 'D:\operator data\model.gguf') 'local_path_positive'
foreach($bad in @('relative.gguf','\\server\share\model.gguf','D:\model.gguf:stream','D:\wild*.gguf',"D:\line`nmodel.gguf",'D:\quote".gguf')){Reject {Get-ProcessorFullPath $bad} 'local_path_negative'}

$arguments=@(Get-ProcessorArguments 'D:\operator data\model.gguf' $digest 2)
foreach($flag in @('--offline','--no-webui','--no-agent','--no-ui-mcp-proxy','--no-slots','--no-models-autoload','--no-mmproj','--no-cache-prompt','--no-cache-idle-slots','--no-context-shift','--no-cont-batching','--log-disable')){Check ($arguments -ccontains $flag) 'disabled_surface'}
foreach($binding in @(@('--host','127.0.0.1'),@('--port','58102'),@('--cors-origins','http://127.0.0.1:58102'),@('--parallel','1'),@('--ctx-size','20480'),@('--cache-type-k','q8_0'),@('--cache-type-v','q8_0'),@('--cache-ram','0'),@('--ctx-checkpoints','0'),@('--device','none'),@('--gpu-layers','0'),@('--fit','off'),@('--reasoning','off'))){$index=[Array]::IndexOf($arguments,$binding[0]);Check ($index -ge 0 -and $arguments[$index+1] -ceq $binding[1]) 'fixed_binding'}
foreach($flag in @('--props','--tools','--tools-runtime','--mcp-servers-config','--mcp-servers-json','--media-path','--models-dir','--models-preset','--slot-save-path','--api-key','--model-url','--hf-repo','--log-file','--log-prompts-dir','--no-props')){Check (-not ($arguments -ccontains $flag)) 'absent_surface'}
Check ((ConvertTo-ProcessorCommandLine $arguments).Contains('"D:\operator data\model.gguf"')) 'path_quoting'
Reject {Get-ProcessorArguments 'D:\x.gguf' $digest 1} 'threads_lower'
Reject {Get-ProcessorArguments 'D:\x.gguf' $digest 5} 'threads_upper'
Reject {Get-ProcessorArguments "D:\x`".gguf" $digest 2} 'path_quote'
Reject {ConvertTo-ProcessorCommandLine @('x\')} 'trailing_separator'
Reject {ConvertTo-ProcessorCommandLine @("bad`narg")} 'control_argument'

$inherited=@{PATH='unchanged';SystemRoot='unchanged';LLAMA_ARG_MODEL='private';LLAMA_API_KEY='private';HF_TOKEN='private';HUGGINGFACE_HUB_CACHE='private';HTTPS_PROXY='private';HTTP_PROXY='private';ALL_PROXY='private';NO_PROXY='private';GGML_BACKEND_PATH='private';MTMD_BACKEND_DEVICE='private';AWS_ACCESS_KEY_ID='private';SSH_AUTH_SOCK='private';SESSION_VALUE='private';CUSTOM_PASSWORD='private'}
$environment=Get-ProcessorEnvironment $inherited
Check (-not $environment.ContainsKey('PATH') -and -not $environment.ContainsKey('SystemRoot')) 'environment_platform_preserved'
foreach($key in $inherited.Keys){if($key -notin @('PATH','SystemRoot')){Check ($environment.ContainsKey($key) -and $null -eq $environment[$key]) 'environment_removed'}}
Check ($inherited.HF_TOKEN -ceq 'private') 'environment_input_unchanged'

$started=[DateTime]::SpecifyKind([DateTime]'2026-10-21T00:00:00',[DateTimeKind]::Utc)
$record=@{schema=1;kind='stackplore.operator.local-processor';source=$script:ProcessorSource;archive_sha256=$script:ProcessorArchiveSHA256;pid=42;start_utc_ticks=$started.Ticks;executable='D:\owned\llama-server.exe'}
$process=[pscustomobject]@{Id=42;StartTime=$started;Path='D:\owned\llama-server.exe';HasExited=$false}
Check (Test-ProcessorOwner $record $process) 'owner_positive'
foreach($property in @('Id','StartTime','Path','HasExited')){
  $changed=[pscustomobject]@{Id=42;StartTime=$started;Path='D:\owned\llama-server.exe';HasExited=$false}
  switch($property){'Id'{$changed.Id=43};'StartTime'{$changed.StartTime=$started.AddTicks(1)};'Path'{$changed.Path='D:\foreign\llama-server.exe'};'HasExited'{$changed.HasExited=$true}}
  Check (-not (Test-ProcessorOwner $record $changed)) 'owner_negative'
}
foreach($property in @('schema','kind','source','archive_sha256','pid','start_utc_ticks')){$changed=$record.Clone();$changed[$property]='wrong';Check (-not (Test-ProcessorOwner $changed $process)) 'receipt_negative'}

foreach($size in @(0,1,1048576)){
  $stream=[IO.MemoryStream]::new([byte[]]::new($size))
  try {$body=Read-ProcessorBytes $stream ([Threading.CancellationToken]::None);Check ($body.Length -eq $size) 'body_bound_positive'} finally {$stream.Dispose()}
}
$stream=[IO.MemoryStream]::new([byte[]]::new(1048577))
try {Reject {Read-ProcessorBytes $stream ([Threading.CancellationToken]::None)} 'body_bound_negative'} finally {$stream.Dispose()}
$cancel=[Threading.CancellationTokenSource]::new();$cancel.Cancel()
$stream=[IO.MemoryStream]::new([byte[]]::new(1))
try {Reject {Read-ProcessorBytes $stream $cancel.Token} 'body_cancel'} finally {$stream.Dispose();$cancel.Dispose()}
$stream=[IO.MemoryStream]::new()
$stream.SetLength(10)
try {
  Assert-ProcessorStreamSize $stream 10 10;Check $true 'opened_length_exact'
  Reject {Assert-ProcessorStreamSize $stream 1 9} 'opened_length_above'
  Reject {Assert-ProcessorStreamSize $stream 11 20} 'opened_length_below'
  $stream.SetLength(16385)
  Reject {Assert-ProcessorStreamSize $stream 1 16384} 'receipt_opened_length_race'
} finally {$stream.Dispose()}

# Parse the entry point without running any executable, process, listener or HTTP operation.
$tokens=$null;$errors=$null
$ast=[Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot 'local-processor.ps1'),[ref]$tokens,[ref]$errors)
Check ($errors.Count -eq 0) 'script_parse'
$starts=@($ast.FindAll({param($node)$node -is [Management.Automation.Language.CommandAst] -and $node.GetCommandName() -ceq 'Start-Process'},$true))
Check ($starts.Count -eq 1) 'single_launch'
Check ($starts[0].Extent.Text -match '\-WindowStyle Hidden' -and $starts[0].Extent.Text -match '\-Environment \$environment') 'hidden_clean_launch'
$source=[IO.File]::ReadAllText((Join-Path $PSScriptRoot 'local-processor.ps1'))
Check ($source -notmatch '(?i)Remove-Item|Invoke-Expression|cmd(?:\.exe)?\s+/c|powershell(?:\.exe)?\s+-command') 'no_shell_or_delete'
Write-Output ("PASS {0} pure checks; no process or network started" -f $script:Checks)
