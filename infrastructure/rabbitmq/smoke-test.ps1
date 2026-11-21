param([switch] $Restart)
$ErrorActionPreference = 'Stop'
$composePath = Join-Path $PSScriptRoot 'compose.yaml'
$values = @{}
foreach ($line in Get-Content (Join-Path $PSScriptRoot '.env')) {
    if ($line -match '^([A-Z_]+)=(.*)$') {
        $values[$Matches[1]] = $Matches[2]
    }
}
$pair = $values['RABBITMQ_ADMIN_USER'] + ':' +
    $values['RABBITMQ_ADMIN_PASSWORD']
$encoded = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($pair))
$headers = @{ Authorization = "Basic $encoded" }
$base = 'http://127.0.0.1:15672/api'

function Invoke-Broker([string] $Method, [string] $Path, $Body = $null) {
    $options = @{
        Method = $Method
        Uri = "$base/$Path"
        Headers = $headers
        TimeoutSec = 10
    }
    if ($null -ne $Body) {
        $options.ContentType = 'application/json'
        $options.Body = $Body | ConvertTo-Json -Depth 10 -Compress
    }
    $response = Invoke-RestMethod @options
    foreach ($item in $response) { Write-Output $item }
}

function Read-TestMessages([string] $Name, [bool] $Remove = $false) {
    $mode = if ($Remove) { 'ack_requeue_false' } else { 'ack_requeue_true' }
    Invoke-Broker POST "queues/govai/$Name/get" @{
        count = 50
        ackmode = $mode
        encoding = 'auto'
        truncate = 50000
    }
}

# Use only an idle development broker; never drain someone else's work.

$queueNames = @(
    'notifications.account.v1',
    'notifications.security.v1',
    'notifications.retry.v1',
    'notifications.security.retry.v1',
    'notifications.dead.v1'
)
foreach ($name in $queueNames) {
    $queue = Invoke-Broker GET "queues/govai/$name"
    if ($queue.messages -ne 0 -or $queue.consumers -ne 0) {
        throw "Queue $name is busy. Existing work was preserved."
    }
}

$cases = @(
    @('notifications', 'citizen.login.succeeded', 'notifications.security.v1'),
    @('notifications', 'citizen.password.changed', 'notifications.security.v1'),
    @('notifications', 'citizen.email.verified', 'notifications.account.v1'),
    @('notifications.retry', 'security', 'notifications.security.v1'),
    @('notifications.retry', 'retry', 'notifications.account.v1'),
    @('notifications.dead', 'failed', 'notifications.dead.v1')
)
$expected = @{}
foreach ($case in $cases) {
    $payload = @{
        event_id = [guid]::NewGuid().ToString()
        event_type = 'citizen.login.succeeded'
        version = 1
        issuer = 'https://synthetic.invalid/realms/smoke'
        subject = 'synthetic-smoke-user'
        occurred_at = [DateTime]::UtcNow.ToString('o')
        data = @{ smoke_marker = [guid]::NewGuid().ToString() }
    }
    if ($case[1] -like 'citizen.*') {
        $payload.event_type = $case[1]
    }
    elseif ($case[1] -eq 'retry') {
        $payload.event_type = 'citizen.email.verified'
    }
    $json = $payload | ConvertTo-Json -Compress -Depth 5
    if (-not $expected.ContainsKey($case[2])) { $expected[$case[2]] = @() }
    $expected[$case[2]] += @($json)
    $published = Invoke-Broker POST "exchanges/govai/$($case[0])/publish" @{
        properties = @{ delivery_mode = 2; content_type = 'application/json' }
        routing_key = $case[1]
        payload = $json
        payload_encoding = 'string'
    }
    if (-not $published.routed) { throw 'Test event was not routed.' }
}

if ($Restart) {
    docker compose -f $composePath restart rabbitmq
    if ($LASTEXITCODE -ne 0) { throw 'Broker restart failed.' }
    docker compose -f $composePath up -d --wait rabbitmq
    if ($LASTEXITCODE -ne 0) { throw 'Broker did not become healthy.' }
}

# Wait for both retry timers, keeping inspected messages in their queues.

$deadline = [DateTime]::UtcNow.AddSeconds(60)
$complete = $false
while ([DateTime]::UtcNow -lt $deadline) {
    $complete = $true
    foreach ($name in $expected.Keys) {
        $messages = @(Read-TestMessages $name)
        foreach ($message in $messages) {
            if ($message.payload -notin $expected[$name]) {
                throw "Unexpected message in $name; all inspected messages were requeued."
            }
        }
        if ($messages.Count -ne @($expected[$name]).Count) {
            $complete = $false
        }
    }
    if ($complete) { break }
    Start-Sleep -Seconds 2
}
if (-not $complete) {
    throw 'Routing or retry check timed out. Test messages were preserved for inspection.'
}
foreach ($name in $expected.Keys) {
    $null = Read-TestMessages $name $true
}

# Management requests without credentials must fail.

$unauthorized = $false
try {
    $null = Invoke-RestMethod "$base/overview" -TimeoutSec 10
}
catch {
    $unauthorized = [int]$_.Exception.Response.StatusCode -eq 401
}
if (-not $unauthorized) { throw 'Unauthenticated request was not rejected.' }
Write-Output 'PASS: security/normal isolation, retry return paths, dead-letter routing.'
Write-Output 'PASS: management authentication.'
if ($Restart) { Write-Output 'PASS: messages survived broker restart.' }
