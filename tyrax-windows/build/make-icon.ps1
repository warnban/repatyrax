#Requires -Version 5.1
<#
.SYNOPSIS
    Builds a multi-resolution Windows .ico from the TYRAX master PNG.

.DESCRIPTION
    Squares the master image on a black canvas (brand background), then renders
    256/128/64/48/32/16 px frames as PNG-compressed entries packed into a single
    .ico. PNG-in-ICO is supported on Windows Vista+ (we target Win10+). Output is
    consumed by the WPF <ApplicationIcon>, the window/tray icon and the installer.

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File build\make-icon.ps1
#>
[CmdletBinding()]
param(
    [string]$Master,
    [string]$OutIco
)

$ErrorActionPreference = "Stop"
$root = Resolve-Path "$PSScriptRoot\.."
if ([string]::IsNullOrWhiteSpace($Master)) { $Master = Join-Path $root "src\Tyrax.App\Assets\tyrax-icon-master.png" }
if ([string]::IsNullOrWhiteSpace($OutIco)) { $OutIco = Join-Path $root "src\Tyrax.App\Assets\tyrax.ico" }

Add-Type -AssemblyName System.Drawing

Write-Host "Icon: $Master -> $OutIco" -ForegroundColor Cyan

$src = [System.Drawing.Image]::FromFile($Master)
try {
    $side = [Math]::Max($src.Width, $src.Height)
    $square = New-Object System.Drawing.Bitmap $side, $side
    $g = [System.Drawing.Graphics]::FromImage($square)
    $g.Clear([System.Drawing.Color]::Black)
    $g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $g.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
    $x = [int](($side - $src.Width) / 2)
    $y = [int](($side - $src.Height) / 2)
    $g.DrawImage($src, $x, $y, $src.Width, $src.Height)
    $g.Dispose()
}
finally {
    $src.Dispose()
}

$sizes = 256, 128, 64, 48, 32, 16
$pngs = @()
foreach ($s in $sizes) {
    $bmp = New-Object System.Drawing.Bitmap $s, $s
    $gg = [System.Drawing.Graphics]::FromImage($bmp)
    $gg.Clear([System.Drawing.Color]::Black)
    $gg.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $gg.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
    $gg.DrawImage($square, 0, 0, $s, $s)
    $gg.Dispose()
    $ms = New-Object System.IO.MemoryStream
    $bmp.Save($ms, [System.Drawing.Imaging.ImageFormat]::Png)
    $pngs += , ($ms.ToArray())
    $bmp.Dispose()
    $ms.Dispose()
}
$square.Dispose()

$count = $pngs.Count
$out = New-Object System.IO.MemoryStream
$bw = New-Object System.IO.BinaryWriter $out
# ICONDIR
$bw.Write([UInt16]0)      # reserved
$bw.Write([UInt16]1)      # type = icon
$bw.Write([UInt16]$count) # image count
# ICONDIRENTRY x count
$offset = 6 + (16 * $count)
for ($i = 0; $i -lt $count; $i++) {
    $s = $sizes[$i]
    $data = $pngs[$i]
    $dim = if ($s -ge 256) { 0 } else { $s }  # 0 means 256 in the ICO spec
    $bw.Write([Byte]$dim)      # width
    $bw.Write([Byte]$dim)      # height
    $bw.Write([Byte]0)         # palette colors
    $bw.Write([Byte]0)         # reserved
    $bw.Write([UInt16]1)       # color planes
    $bw.Write([UInt16]32)      # bits per pixel
    $bw.Write([UInt32]$data.Length)
    $bw.Write([UInt32]$offset)
    $offset += $data.Length
}
foreach ($data in $pngs) { $bw.Write($data) }
$bw.Flush()
[System.IO.File]::WriteAllBytes($OutIco, $out.ToArray())
$bw.Dispose()
$out.Dispose()

$fi = Get-Item $OutIco
Write-Host ("Done. " + $fi.FullName + " (" + [math]::Round($fi.Length/1KB,1) + " KB, " + $count + " frames)") -ForegroundColor Green
