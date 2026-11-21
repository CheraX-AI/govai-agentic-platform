$ErrorActionPreference = 'Stop'
$environmentPath = Join-Path $PSScriptRoot '.env'
$definitionsPath = Join-Path $PSScriptRoot '.env.definitions.json'

function New-LocalPassword {
    $bytes = New-Object byte[] 32
    $random = [Security.Cryptography.RandomNumberGenerator]::Create()
    try {
        $random.GetBytes($bytes)
        return [Convert]::ToBase64String($bytes)
    }
    finally {
        $random.Dispose()
    }
}

function Get-PasswordHash([string] $Password) {

    # RabbitMQ expects a four-byte salt followed by the SHA-256 digest.

    $salt = New-Object byte[] 4
    $random = [Security.Cryptography.RandomNumberGenerator]::Create()
    $sha = [Security.Cryptography.SHA256]::Create()
    try {
        $random.GetBytes($salt)
        $passwordBytes = [Text.Encoding]::UTF8.GetBytes($Password)
        $digest = $sha.ComputeHash([byte[]]($salt + $passwordBytes))
        return [Convert]::ToBase64String([byte[]]($salt + $digest))
    }
    finally {
        $random.Dispose()
        $sha.Dispose()
    }
}

# Preserve the credentials when setup is run again.

if (-not (Test-Path -LiteralPath $environmentPath)) {
    $settings = @(
        'RABBITMQ_ADMIN_USER=govai_admin'
        "RABBITMQ_ADMIN_PASSWORD=$(New-LocalPassword)"
        'RABBITMQ_PUBLISHER_USER=govai_auth'
        "RABBITMQ_PUBLISHER_PASSWORD=$(New-LocalPassword)"
        'RABBITMQ_CONSUMER_USER=govai_notifications'
        "RABBITMQ_CONSUMER_PASSWORD=$(New-LocalPassword)"
        'RABBITMQ_VHOST=govai'
    )
    [IO.File]::WriteAllLines($environmentPath, $settings)
}

$values = @{}
foreach ($line in Get-Content -LiteralPath $environmentPath) {
    if ($line -match '^([A-Z_]+)=(.*)$') {
        $values[$Matches[1]] = $Matches[2]
    }
}
if ($values['RABBITMQ_VHOST'] -ne 'govai') {
    throw 'This topology requires RABBITMQ_VHOST=govai.'
}
$topology = Get-Content (Join-Path $PSScriptRoot 'definitions.json') -Raw |
    ConvertFrom-Json
$users = @()
$permissions = @()

foreach ($role in @('ADMIN', 'PUBLISHER', 'CONSUMER')) {
    $username = $values['RABBITMQ_' + $role + '_USER']
    $password = $values['RABBITMQ_' + $role + '_PASSWORD']
    if (-not $username -or -not $password -or $password -eq 'generated_locally') {
        throw "Missing local credentials for $role."
    }
    $tags = @()
    $configure = '^$'
    $write = '^notifications$'
    $read = '^$'
    if ($role -eq 'ADMIN') {
        $tags = @('administrator')
        $configure = '.*'
        $write = '.*'
        $read = '.*'
    }
    elseif ($role -eq 'CONSUMER') {
        $write = '^notifications(\.retry|\.dead)?$'
        $read = '^notifications\.(account|security)\.v1$'
    }
    $users += @{
        name = $username
        password_hash = Get-PasswordHash $password
        hashing_algorithm = 'rabbit_password_hashing_sha256'
        tags = $tags
    }
    $permissions += @{
        user = $username
        vhost = 'govai'
        configure = $configure
        write = $write
        read = $read
    }
}

# Only the ignored local file contains credential hashes.

$topology | Add-Member -NotePropertyName users -NotePropertyValue $users
$topology | Add-Member -NotePropertyName permissions -NotePropertyValue $permissions
$json = $topology | ConvertTo-Json -Depth 20
[IO.File]::WriteAllText($definitionsPath, $json)
Write-Output 'RabbitMQ local credentials and startup definitions are ready.'
