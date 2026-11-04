$ErrorActionPreference = 'Stop'
$taskTarget = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../.env.registration'))
if (Test-Path -LiteralPath $taskTarget) { throw 'Private registration settings already exist. Preserve the stable keys; edit the existing file.' }
function New-RegistrationKey {
    $taskBytes = New-Object byte[] 32
    $taskRng = [Security.Cryptography.RandomNumberGenerator]::Create()
    try { $taskRng.GetBytes($taskBytes) } finally { $taskRng.Dispose() }
    return [Convert]::ToBase64String($taskBytes)
}
$taskFingerprint = New-RegistrationKey
$taskRate = New-RegistrationKey
$taskContent = @"
# Private local configuration. Complete HTTPS/client settings before enabling.
REGISTRATION_ENABLED=false
REGISTRATION_KEYCLOAK_URL=
REGISTRATION_REALM=GovAI
REGISTRATION_CLIENT_ID=
REGISTRATION_CLIENT_SECRET=
REGISTRATION_GROUP_PATH=/ga48-registration
REGISTRATION_VERIFICATION_CLIENT_ID=
REGISTRATION_VERIFICATION_REDIRECT=
# Optional additional trusted CA bundle for the HTTPS Keycloak server.
REGISTRATION_CA_FILE=
API_TLS_CERT_FILE=
API_TLS_KEY_FILE=
REGISTRATION_FINGERPRINT_KEY=$taskFingerprint
REGISTRATION_RATE_KEY=$taskRate
"@
[IO.File]::WriteAllText($taskTarget, $taskContent)
Write-Host 'Created private registration settings with independent keys. Registration remains disabled until HTTPS/client configuration is completed.'
