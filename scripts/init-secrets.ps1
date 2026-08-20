$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$secretDirectory = Join-Path $root 'secrets'
New-Item -ItemType Directory -Force -Path $secretDirectory | Out-Null

function New-RandomSecret {
    $bytes = [byte[]]::new(32)
    [System.Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
    return [Convert]::ToBase64String($bytes).TrimEnd('=').Replace('+', '-').Replace('/', '_')
}

foreach ($name in @('postgres_password', 'redis_password', 'admin_password')) {
    $path = Join-Path $secretDirectory $name
    if (-not (Test-Path -LiteralPath $path)) {
        [System.IO.File]::WriteAllText($path, (New-RandomSecret), [System.Text.UTF8Encoding]::new($false))
        Write-Host "Created secrets/$name"
    } else {
        Write-Host "Kept existing secrets/$name"
    }
}

Write-Host 'Secrets are ready. The generated owner password is stored in secrets/admin_password.'
