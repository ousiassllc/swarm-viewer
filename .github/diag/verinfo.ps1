param([string]$Path)
Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;
public static class Ver {
  [DllImport("version.dll", CharSet=CharSet.Unicode)] public static extern int GetFileVersionInfoSize(string f, out int h);
  [DllImport("version.dll", CharSet=CharSet.Unicode)] public static extern bool GetFileVersionInfo(string f, int h, int len, byte[] data);
  [DllImport("version.dll", CharSet=CharSet.Unicode)] public static extern bool VerQueryValue(byte[] block, string sub, out IntPtr buf, out int len);
}
"@
$full = (Resolve-Path $Path).Path
$h = 0
$size = [Ver]::GetFileVersionInfoSize($full, [ref]$h)
"size=$size"
$data = New-Object byte[] $size
[void][Ver]::GetFileVersionInfo($full, 0, $size, $data)
$p = [IntPtr]::Zero; $l = 0
if ([Ver]::VerQueryValue($data, '\VarFileInfo\Translation', [ref]$p, [ref]$l)) {
  $b = New-Object byte[] $l; [Runtime.InteropServices.Marshal]::Copy($p, $b, 0, $l)
  "Translation bytes: " + (($b | % { '{0:X2}' -f $_ }) -join ' ')
  $lang = '{0:X4}{1:X4}' -f [BitConverter]::ToUInt16($b,0), [BitConverter]::ToUInt16($b,2)
} else { "no Translation"; $lang = '040904B0' }
foreach ($lg in @($lang, '040904B0', '040904E4', '000004B0')) {
  foreach ($k in 'ProductVersion','FileVersion','ProductName','CompanyName') {
    $p = [IntPtr]::Zero; $l = 0
    if ([Ver]::VerQueryValue($data, "\StringFileInfo\$lg\$k", [ref]$p, [ref]$l)) { "$lg $k = " + [Runtime.InteropServices.Marshal]::PtrToStringUni($p) } else { "$lg $k : (none)" }
  }
}
$fi = [System.Diagnostics.FileVersionInfo]::GetVersionInfo($full)
".NET ProductVersion='$($fi.ProductVersion)' FileVersion='$($fi.FileVersion)' Raw=$($fi.FileVersionRaw) PSVersion=$($PSVersionTable.PSVersion)"
