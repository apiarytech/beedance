# esp32.ps1 [-Port COM6] [-Seconds 180]: resets an ESP32 through RTS and
# prints its output until "bench: done". For a board whose BOOT button must
# be held to flash: release it, then run this, so the board does not start
# in download mode. Elsewhere, `tinygo flash ... -monitor` shows the output.
param([string]$Port = "COM6", [int]$Seconds = 180)
$p = New-Object System.IO.Ports.SerialPort $Port, 115200, None, 8, One
$p.ReadTimeout = 500
$p.DtrEnable = $false
$p.RtsEnable = $true   # RTS drives EN low: reset
$p.Open()
Start-Sleep -Milliseconds 100
$p.RtsEnable = $false  # release EN: boot
$deadline = (Get-Date).AddSeconds($Seconds)
try {
  while ((Get-Date) -lt $deadline) {
    try {
      $line = $p.ReadLine().TrimEnd("`r")
      Write-Output $line
      if ($line -eq "bench: done" -or $line -like "panic*" -or $line -like "fatal error*") { break }
    } catch [System.TimeoutException] {}
  }
} finally {
  $p.Close()
}
