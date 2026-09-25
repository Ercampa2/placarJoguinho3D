# Cria no Menu Iniciar um atalho para o agua.ps1 que dispara com Ctrl+Alt+A.
# Rode uma vez: botao direito neste arquivo -> "Executar com o PowerShell".
# Deixe os dois arquivos na mesma pasta, que nao deve mudar depois (o atalho
# aponta para ela). Para usar outra tecla, troque $Tecla e rode de novo.

$Tecla = "CTRL+ALT+A"

$script = Join-Path $PSScriptRoot "agua.ps1"
$destino = Join-Path ([Environment]::GetFolderPath("Programs")) "Gole de agua.lnk"

$shell = New-Object -ComObject WScript.Shell
$atalho = $shell.CreateShortcut($destino)
$atalho.TargetPath = Join-Path $env:SystemRoot "System32\WindowsPowerShell\v1.0\powershell.exe"
$atalho.Arguments = "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$script`""
$atalho.WorkingDirectory = $PSScriptRoot
# 7 = abre minimizado sem ativar a janela, e o -WindowStyle Hidden esconde o
# resto: o PowerShell roda sem tirar o foco do que voce esta usando.
$atalho.WindowStyle = 7
$atalho.Hotkey = $Tecla
$atalho.Save()

Write-Host "Atalho criado: $destino"
Write-Host "Aperte $Tecla para registrar um gole (pode levar 1 ou 2 segundos)."
Write-Host "Se a tecla nao funcionar, saia e entre de novo no Windows."
Read-Host "Enter para fechar"
