param([switch]$Publisher)

$ErrorActionPreference = 'Stop'

# All tests use disposable containers without publishing any host ports.

$fixture = 'govai-auth-outbox-check-' + [Guid]::NewGuid().ToString('N').Substring(0, 10)
$broker = $fixture + '-rabbitmq'
$backend = (Resolve-Path (Join-Path $PSScriptRoot '../../..')).Path
$mount = "type=bind,source=$backend,target=/src,readonly"
$savedEnvironment = @{}
$environmentNames = @(
    'POSTGRES_PASSWORD', 'GOVAI_OUTBOX_TEST_PASSWORD', 'GOVAI_OUTBOX_RESTART_PROBE',
    'RABBITMQ_DEFAULT_PASS', 'GOVAI_OUTBOX_PUBLISHER_TEST', 'GOVAI_OUTBOX_BROKER_PHASE'
)
foreach ($name in $environmentNames) {
    $savedEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}
$created = $false
$brokerCreated = $false
$generator = [Security.Cryptography.RandomNumberGenerator]::Create()

function Wait-TestDatabase {
    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        $probe = @(
            'exec', $fixture, 'pg_isready', '-h', '127.0.0.1',
            '-U', 'outbox_owner', '-d', 'govai_auth_outbox_test'
        )
        docker @probe *> $null
        if ($LASTEXITCODE -eq 0) { return }
        Start-Sleep -Seconds 1
    }
    throw 'Disposable outbox database did not become ready.'
}

function Wait-TestBroker {
    $probe = $checks.Clone()
    $probe[$probe.Count - 1] = '^TestNotificationPublisherReady$'
    docker @probe
    if ($LASTEXITCODE -ne 0) {
        throw 'Disposable publisher broker did not become ready.'
    }
}

try {
    $bytes = New-Object byte[] 32
    $generator.GetBytes($bytes)
    $env:POSTGRES_PASSWORD = [Convert]::ToBase64String($bytes)
    $env:GOVAI_OUTBOX_TEST_PASSWORD = $env:POSTGRES_PASSWORD
    $env:GOVAI_OUTBOX_RESTART_PROBE = '0'
    $env:GOVAI_OUTBOX_PUBLISHER_TEST = '0'
    $env:GOVAI_OUTBOX_BROKER_PHASE = ''
    $arguments = @(
        'run', '--detach', '--rm', '--name', $fixture,
        '-e', 'POSTGRES_PASSWORD', '-e', 'POSTGRES_USER=outbox_owner',
        '-e', 'POSTGRES_DB=govai_auth_outbox_test',
        '-e', 'POSTGRES_INITDB_ARGS=--auth-host=scram-sha-256', 'postgres:16.14'
    )
    docker @arguments | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Disposable outbox database could not start.' }
    $created = $true
    Wait-TestDatabase
    $checks = @(
        'run', '--rm', '--network', "container:$fixture", '--mount', $mount,
        '-w', '/src/services/auth-service',
        '-e', 'GOVAI_OUTBOX_TEST_PASSWORD', '-e', 'GOVAI_OUTBOX_RESTART_PROBE',
        '-e', 'GOVAI_OUTBOX_PUBLISHER_TEST', '-e', 'GOVAI_OUTBOX_BROKER_PHASE',
        'govai-auth-checks:local', 'go', 'test', '-count=1', '-v',
        './internal/external/postgres', '-run', '^TestNotificationOutbox'
    )
    docker @checks
    if ($LASTEXITCODE -ne 0) { throw 'Outbox storage tests failed.' }

    # Restart only the fixture; its anonymous volume must preserve pending events.

    docker restart $fixture | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Disposable outbox database restart failed.' }
    Wait-TestDatabase
    $env:GOVAI_OUTBOX_RESTART_PROBE = '1'
    $checks[$checks.Count - 1] = '^TestNotification(OutboxRestart|AccountCheckpointsRestart)$'
    docker @checks
    if ($LASTEXITCODE -ne 0) { throw 'Outbox restart durability test failed.' }

    $runtimeChecks = $checks.Clone()
    $runtimeChecks[$runtimeChecks.Count - 3] = './internal/app'
    $runtimeChecks[$runtimeChecks.Count - 1] = '^TestNotificationRuntimeDatabase$'
    docker @runtimeChecks
    if ($LASTEXITCODE -ne 0) { throw 'Notification runtime integration test failed.' }
    if ($Publisher) {
        $env:RABBITMQ_DEFAULT_PASS = $env:POSTGRES_PASSWORD
        $arguments = @(
            'run', '--detach', '--name', $broker,
            '--network', "container:$fixture",
            '-e', 'RABBITMQ_DEFAULT_USER=outbox_admin', '-e', 'RABBITMQ_DEFAULT_PASS',
            '-e', 'RABBITMQ_DEFAULT_VHOST=govai', '-e', 'RABBITMQ_NODENAME=rabbit@localhost',
            '-e', 'RABBITMQ_SERVER_ADDITIONAL_ERL_ARGS=+S 2:2 +SDcpu 1 +SDio 1',
            'rabbitmq:4.4.0-management'
        )
        docker @arguments | Out-Null
        if ($LASTEXITCODE -ne 0) { throw 'Disposable publisher broker could not start.' }
        $brokerCreated = $true
        $env:GOVAI_OUTBOX_PUBLISHER_TEST = '1'
        Wait-TestBroker
        $checks[$checks.Count - 1] = '^TestNotificationPublisherBroker$'
        docker @checks
        if ($LASTEXITCODE -ne 0) { throw 'Publisher routing/confirmation tests failed.' }

        # Exercise a real outage of this test broker, then retry the same saved ID.

        docker stop --time 5 $broker | Out-Null
        if ($LASTEXITCODE -ne 0) { throw 'Disposable publisher broker stop failed.' }
        $env:GOVAI_OUTBOX_BROKER_PHASE = 'down'
        $checks[$checks.Count - 1] = '^TestNotificationPublisherOutage$'
        docker @checks
        if ($LASTEXITCODE -ne 0) { throw 'Publisher outage test failed.' }
        docker start $broker | Out-Null
        if ($LASTEXITCODE -ne 0) { throw 'Disposable publisher broker restart failed.' }
        Wait-TestBroker
        $env:GOVAI_OUTBOX_BROKER_PHASE = 'up'
        docker @checks
        if ($LASTEXITCODE -ne 0) { throw 'Publisher recovery test failed.' }
    }
} finally {

    # Remove only these fixtures and their anonymous volumes; restore environment.

    if ($brokerCreated) { docker container rm --force --volumes $broker | Out-Null }
    if ($created) { docker container rm --force --volumes $fixture | Out-Null }
    foreach ($name in $environmentNames) {
        [Environment]::SetEnvironmentVariable($name, $savedEnvironment[$name], 'Process')
    }
    $generator.Dispose()
}
