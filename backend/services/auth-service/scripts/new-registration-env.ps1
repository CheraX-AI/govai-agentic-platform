$ErrorActionPreference = 'Stop'
# Adds the Registration section to this service's single .env (created if missing).
$taskTarget = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../.env'))
$taskExisting = if (Test-Path -LiteralPath $taskTarget) { [IO.File]::ReadAllText($taskTarget) } else { '' }
if ($taskExisting -match '(?m)^\s*REGISTRATION_[A-Z_]+\s*=') { throw 'Private registration settings already exist in .env. Preserve the stable keys; edit the existing lines.' }
function New-RegistrationKey {
    $taskBytes = New-Object byte[] 32
    $taskRng = [Security.Cryptography.RandomNumberGenerator]::Create()
    try { $taskRng.GetBytes($taskBytes) } finally { $taskRng.Dispose() }
    return [Convert]::ToBase64String($taskBytes)
}
$taskFingerprint = New-RegistrationKey
$taskRate = New-RegistrationKey
$taskContent = @"

# -- Registration
# Complete the HTTPS/client settings (go run ./tools/dev-auth configure) before enabling.
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
REGISTRATION_FINGERPRINT_KEY=$taskFingerprint
REGISTRATION_RATE_KEY=$taskRate
"@
$taskText = (($taskExisting -replace "`r`n", "`n").TrimEnd() + "`n" + ($taskContent -replace "`r`n", "`n")).TrimStart()
[IO.File]::WriteAllText($taskTarget, $taskText)
Write-Host 'Added private registration settings with independent keys to .env. Registration remains disabled until HTTPS/client configuration is completed.'
