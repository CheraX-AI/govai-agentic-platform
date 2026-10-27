$ErrorActionPreference = 'Stop'
$mobileRoot = Split-Path $PSScriptRoot -Parent
$assetRoot = Join-Path $mobileRoot 'assets/figma/welcome'
$scratchRoot = Join-Path $mobileRoot '.cache/welcome-icons'
New-Item -ItemType Directory -Force $scratchRoot | Out-Null
$chromePath = 'C:/Program Files/Google/Chrome/Application/chrome.exe'
Add-Type -AssemblyName System.Drawing
foreach ($name in @('next', 'globe', 'chevron')) {
  $size = if ($name -eq 'chevron') { 42 } else { 48 }
  $svgPath = Join-Path $assetRoot "$name.svg"
  $htmlPath = Join-Path $scratchRoot "$name.html"
  $capturePath = Join-Path $scratchRoot "$name.png"
  $svgUri = ([System.Uri]$svgPath).AbsoluteUri
  Set-Content -LiteralPath $htmlPath -Encoding UTF8 -Value "<html><style>html,body{margin:0;background:transparent}img{display:block;width:${size}px;height:${size}px}</style><img src='$svgUri'></html>"
  $chromeArguments = @('--headless', '--no-sandbox', '--disable-gpu', '--hide-scrollbars', '--no-first-run', '--no-default-browser-check', '--default-background-color=00000000', "--user-data-dir=`"$scratchRoot/profile-$name`"", "--screenshot=`"$capturePath`"", '--window-size=500,500', '--timeout=10000', "`"$(([System.Uri]$htmlPath).AbsoluteUri)`"")
  Start-Process -FilePath $chromePath -ArgumentList $chromeArguments -WindowStyle Hidden -Wait
  if (!(Test-Path $capturePath)) { throw "Missing capture: $name" }
  $source = [System.Drawing.Bitmap]::FromFile($capturePath)
  $crop = $source.Clone([System.Drawing.Rectangle]::new(0, 0, $size, $size), [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
  $crop.Save((Join-Path $assetRoot "$name.png"), [System.Drawing.Imaging.ImageFormat]::Png)
  $crop.Dispose()
  $source.Dispose()
}
