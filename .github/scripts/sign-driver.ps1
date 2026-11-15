<#
.SYNOPSIS
    Creates a self-signed test certificate and signs the keyboardctl driver.

.DESCRIPTION
    Windows 10/11 x64 refuses to load an unsigned kernel-mode driver, and since
    Windows 10 1507 an unsigned driver is rejected outright when Memory
    Integrity / HVCI is enabled.  This script therefore:

      1. creates a self-signed code-signing certificate,
      2. exports it as both .cer (shipped to users) and .pfx (kept in CI),
      3. trusts it locally so signtool can use it,
      4. signs the .sys with signtool.

    Users then import the .cer and enable test-signing (install.bat does this).

.NOTES
    This produces a TEST signature only.  Public distribution still requires a
    production signature from Microsoft (an EV certificate + the Hardware
    Dev Center portal).
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$DriverPath,
    [Parameter(Mandatory = $true)][string]$OutDir,
    [string]$Subject  = 'CN=keyboardctl test signing',
    [string]$Password = 'keyboardctl'
)

$ErrorActionPreference = 'Stop'

if (-not (Test-Path -LiteralPath $DriverPath)) {
    throw "Driver not found: $DriverPath"
}

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

$cert = New-SelfSignedCertificate `
    -Type CodeSigningCert `
    -Subject $Subject `
    -CertStoreLocation 'Cert:\CurrentUser\My' `
    -KeyUsage DigitalSignature `
    -KeyExportPolicy Exportable `
    -KeyAlgorithm RSA `
    -KeyLength 2048 `
    -NotAfter (Get-Date).AddYears(10)

$cerPath = Join-Path $OutDir 'keyboardctl-test.cer'
$pfxPath = Join-Path $OutDir 'keyboardctl-test.pfx'
$securePassword = ConvertTo-SecureString -String $Password -Force -AsPlainText

Export-Certificate   -Cert $cert -FilePath $cerPath | Out-Null
Export-PfxCertificate -Cert $cert -FilePath $pfxPath -Password $securePassword | Out-Null

Import-Certificate -FilePath $cerPath -CertStoreLocation 'Cert:\LocalMachine\Root'            | Out-Null
Import-Certificate -FilePath $cerPath -CertStoreLocation 'Cert:\LocalMachine\TrustedPublisher' | Out-Null

$signtool = Get-ChildItem -Path "${env:ProgramFiles(x86)}\Windows Kits\10\bin" `
        -Recurse -Filter 'signtool.exe' -ErrorAction SilentlyContinue |
    Where-Object { $_.FullName -match '\\x64\\' } |
    Select-Object -First 1 -ExpandProperty FullName

if (-not $signtool) {
    throw 'signtool.exe not found. Install the Windows SDK.'
}

& $signtool sign /v /fd sha256 /f $pfxPath /p $Password $DriverPath
if ($LASTEXITCODE -ne 0) {
    throw "signtool failed with exit code $LASTEXITCODE"
}

Write-Host "Signed:     $DriverPath"
Write-Host "Certificate: $cerPath"
