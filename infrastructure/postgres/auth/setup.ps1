$ErrorActionPreference = 'Stop'
$taskService = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../../../backend/services/auth-service'))
$taskComposeEnv = Join-Path $PSScriptRoot '.env'
# The service keeps all runtime settings in one .env; the owner login goes to a separate .env.migrate.
$taskServiceEnv = Join-Path $taskService '.env'
$taskAdminEnv = Join-Path $taskService '.env.migrate'
$taskHasDatabase = (Test-Path -LiteralPath $taskServiceEnv) -and (Select-String -LiteralPath $taskServiceEnv -Pattern '^\s*(DATABASE_URL|PROFILE_DATA_KEY|NIC_INDEX_KEY)\s*=\s*\S' -Quiet)
if ((Test-Path -LiteralPath $taskComposeEnv) -or $taskHasDatabase -or (Test-Path -LiteralPath $taskAdminEnv)) {
    throw 'Local database settings already exist. Keep them: do not regenerate passwords or encryption/index keys for an existing volume.'
}
function New-TaskSecret {
    $taskBytes = New-Object byte[] 32
    $taskRng = [Security.Cryptography.RandomNumberGenerator]::Create()
    try { $taskRng.GetBytes($taskBytes) } finally { $taskRng.Dispose() }
    return $taskBytes
}
$taskOwner = ([BitConverter]::ToString((New-TaskSecret))).Replace('-', '').ToLowerInvariant()
$taskApp = ([BitConverter]::ToString((New-TaskSecret))).Replace('-', '').ToLowerInvariant()
$taskDataKey = [Convert]::ToBase64String((New-TaskSecret))
$taskNICKey = [Convert]::ToBase64String((New-TaskSecret))
[IO.File]::WriteAllText($taskComposeEnv, "AUTH_DB_OWNER_PASSWORD=$taskOwner`nAUTH_DB_APP_PASSWORD=$taskApp`n")
$taskDatabase = "`n# -- Database`nDATABASE_URL=postgres://govai_auth_app:${taskApp}@127.0.0.1:55432/govai_auth?sslmode=disable`nPROFILE_DATA_KEY=$taskDataKey`nNIC_INDEX_KEY=$taskNICKey`n"
$taskExisting = if (Test-Path -LiteralPath $taskServiceEnv) { [IO.File]::ReadAllText($taskServiceEnv) } else { '' }
# Drop empty placeholder lines for these keys, then add the generated section at the end.
$taskKept = ($taskExisting -split "`r?`n") | Where-Object { $_ -notmatch '^\s*(DATABASE_URL|PROFILE_DATA_KEY|NIC_INDEX_KEY)\s*=\s*$' }
[IO.File]::WriteAllText($taskServiceEnv, (($taskKept -join "`n").TrimEnd() + "`n" + $taskDatabase).TrimStart())
[IO.File]::WriteAllText($taskAdminEnv, "MIGRATION_DATABASE_URL=postgres://govai_auth_owner:${taskOwner}@127.0.0.1:55432/govai_auth?sslmode=disable`n")
Write-Host 'Created private local database settings. Secrets were not printed. Keep the settings files and database volume together.'
