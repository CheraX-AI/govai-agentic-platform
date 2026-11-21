$ErrorActionPreference = 'Stop'
$composePath = Join-Path $PSScriptRoot 'compose.yaml'
$settings = @{}
foreach ($line in Get-Content (Join-Path $PSScriptRoot '.env')) {
    if ($line -match '^([A-Z_]+)=(.*)$') {
        $settings[$Matches[1]] = $Matches[2]
    }
}
$pair = $settings['RABBITMQ_ADMIN_USER'] + ':' +
    $settings['RABBITMQ_ADMIN_PASSWORD']
$encoded = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($pair))
$headers = @{ Authorization = "Basic $encoded" }
$base = 'http://127.0.0.1:15672/api'
$bindingsPath = "$base/bindings/govai/e/notifications/q/notifications.account.v1"
$bindings = Invoke-RestMethod $bindingsPath -Headers $headers
$oldBindings = @($bindings | Where-Object {
    $_.routing_key -in @('citizen.login.succeeded', 'citizen.password.changed')
})

# Drain the old route before changing it; no queue or stored message is deleted.

if ($oldBindings.Count -gt 0) {
    foreach ($name in @('notifications.account.v1', 'notifications.retry.v1')) {
        $queue = Invoke-RestMethod "$base/queues/govai/$name" -Headers $headers
        if ($queue.messages -ne 0 -or $queue.consumers -ne 0) {
            throw 'Old queues are busy. Stop producers and drain them before migration.'
        }
    }
}

# Definitions import adds routes but does not remove superseded bindings.

& (Join-Path $PSScriptRoot 'setup.ps1')
$arguments = @(
    'compose', '-f', $composePath, 'exec', '-T', 'rabbitmq',
    'rabbitmqctl', 'import_definitions', '/etc/rabbitmq/definitions.json'
)
docker @arguments
if ($LASTEXITCODE -ne 0) { throw 'Topology import failed.' }

# Confirm the replacement route exists before removing the superseded route.

$ready = $false
for ($attempt = 0; $attempt -lt 30; $attempt++) {
    try {
        $route = "$base/bindings/govai/e/notifications/q/notifications.security.v1"
        $newBindings = Invoke-RestMethod $route -Headers $headers
        $keys = @($newBindings | ForEach-Object { $_.routing_key })
        $ready = 'citizen.login.succeeded' -in $keys -and
            'citizen.password.changed' -in $keys
    }
    catch {
        $ready = $false
    }
    if ($ready) { break }
    Start-Sleep -Seconds 1
}
if (-not $ready) { throw 'Security route is not ready. Old bindings were preserved.' }

foreach ($binding in $oldBindings) {
    $key = [Uri]::EscapeDataString($binding.properties_key)
    $null = Invoke-RestMethod "$bindingsPath/$key" -Method Delete -Headers $headers
}
Write-Output 'Security and normal account routing are configured.'
