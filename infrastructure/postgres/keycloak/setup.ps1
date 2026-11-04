$ErrorActionPreference = 'Stop'
$taskComposeEnv = Join-Path $PSScriptRoot '.env'
$taskKeycloakEnv = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../../keycloak/.env.postgres'))
if ((Test-Path -LiteralPath $taskComposeEnv) -or (Test-Path -LiteralPath $taskKeycloakEnv)) {
    throw 'Private settings already exist. Preserve credentials for the existing volume; do not regenerate them.'
}
$taskBootstrap = [Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(32)).ToLowerInvariant()
$taskRuntime = [Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(32)).ToLowerInvariant()
$taskUtf8 = [Text.UTF8Encoding]::new($false)
[IO.File]::WriteAllText($taskComposeEnv, "KEYCLOAK_DB_BOOTSTRAP_PASSWORD=$taskBootstrap`nKEYCLOAK_DB_APP_PASSWORD=$taskRuntime`n", $taskUtf8)
[IO.File]::WriteAllText($taskKeycloakEnv, "KEYCLOAK_DB_PASSWORD=$taskRuntime`n", $taskUtf8)
Remove-Variable taskBootstrap,taskRuntime
Write-Host 'Created ignored database credentials; existing Keycloak configuration was not changed.'
