$ErrorActionPreference = 'Stop'

# Isolate the test server from project databases, ports and persistent volumes.

$fixture = 'govai-notification-dbcheck-' + [Guid]::NewGuid().ToString('N').Substring(0, 10)
$backend = (Resolve-Path (Join-Path $PSScriptRoot '../../..')).Path
$mount = "type=bind,source=$backend,target=/src,readonly"
$previousPassword = $env:POSTGRES_PASSWORD
$previousTestPassword = $env:NOTIFICATION_TEST_DB_PASSWORD
$created = $false
$generator = [Security.Cryptography.RandomNumberGenerator]::Create()

try {
    $bytes = New-Object byte[] 32
    $generator.GetBytes($bytes)
    $env:POSTGRES_PASSWORD = [Convert]::ToBase64String($bytes)
    $env:NOTIFICATION_TEST_DB_PASSWORD = $env:POSTGRES_PASSWORD
    $arguments = @(
        'run', '--detach', '--rm', '--name', $fixture,
        '--tmpfs', '/var/lib/postgresql/data',
        '-e', 'POSTGRES_PASSWORD',
        '-e', 'POSTGRES_USER=notification_test',
        '-e', 'POSTGRES_DB=govai_notifications',
        '-e', 'POSTGRES_INITDB_ARGS=--auth-host=scram-sha-256',
        'postgres:16.14'
    )
    docker @arguments | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw 'Disposable database could not start.'
    }
    $created = $true

    # Host authentication must reject wrong passwords, including on loopback.

    $ready = $false
    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        $probe = @(
            'exec', $fixture, 'pg_isready', '-h', '127.0.0.1',
            '-U', 'notification_test', '-d', 'govai_notifications'
        )
        docker @probe *> $null
        if ($LASTEXITCODE -eq 0) {
            $ready = $true
            break
        }
        Start-Sleep -Seconds 1
    }
    if (-not $ready) {
        throw 'Disposable database did not become ready.'
    }
    $checks = @(
        'run', '--rm', '--network', "container:$fixture", '--mount', $mount,
        '-w', '/src/services/notification-service',
        '-e', 'NOTIFICATION_TEST_DB_PASSWORD', 'govai-notification-checks:local',
        'go', 'test', '-count=1', '-v', './internal/external/postgres'
    )
    docker @checks
    if ($LASTEXITCODE -ne 0) {
        throw 'Database connection tests failed.'
    }
} finally {

    # Remove only this script's container; restore the caller's environment.

    if ($created) {
        docker container rm --force $fixture | Out-Null
    }
    $env:POSTGRES_PASSWORD = $previousPassword
    $env:NOTIFICATION_TEST_DB_PASSWORD = $previousTestPassword
    $generator.Dispose()
}
