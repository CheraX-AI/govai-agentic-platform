$ErrorActionPreference = 'Stop'
$service = Split-Path $PSScriptRoot -Parent
$target = Join-Path $service '.env.smtp-sandbox'
$example = Join-Path $service 'smtp-sandbox.env.example'

# Preserve any sandbox credentials already configured locally.

if (Test-Path -LiteralPath $target) {
    Write-Output 'Existing sandbox settings preserved.'
}
else {
    Copy-Item -LiteralPath $example -Destination $target
    Write-Output 'Local sandbox settings created with sending disabled.'
}
Write-Output 'Edit .env.smtp-sandbox privately, then run smtp-check without -send first.'
