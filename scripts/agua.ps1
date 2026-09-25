# Registra um gole de agua no bot (Windows 10).
# Configure as tres linhas abaixo e rode uma vez o instalar-atalho.ps1, que
# cria a tecla de atalho. Usa o curl.exe que ja vem no Windows 10 (1803+).
# Este arquivo fica sem acentos de proposito: o PowerShell 5.1 le arquivos sem
# BOM como ANSI e estragaria qualquer acento escrito aqui.

$BotUrl  = "http://192.168.0.10:8080"   # endereco da maquina do bot
$Token   = "COLE_AQUI_SEU_TOKEN"        # gere com /agua_token no Discord
$Trilhas = @("5min")                    # @("5min"), @("10min") ou @("5min", "10min")

function Send-Gole([string]$Trilha) {
    $saida = [System.IO.Path]::GetTempFileName()
    try {
        # curl.exe, nao curl: no PowerShell 5.1, "curl" e apelido do
        # Invoke-WebRequest. A resposta vai para um arquivo e e lida como
        # UTF-8, para os acentos da mensagem do bot chegarem certos. Sem -f,
        # o curl so falha quando nao consegue falar com o bot.
        & curl.exe -s --max-time 5 -o $saida -X POST -H "Authorization: Bearer $Token" "$BotUrl/gole/$Trilha" | Out-Null
        if ($LASTEXITCODE -ne 0) {
            return "Sem resposta do bot em $BotUrl (erro $LASTEXITCODE do curl)"
        }
        return [System.IO.File]::ReadAllText($saida, [System.Text.Encoding]::UTF8).Trim()
    } catch {
        return "Nao consegui rodar o curl.exe: $($_.Exception.Message)"
    } finally {
        Remove-Item $saida -ErrorAction SilentlyContinue
    }
}

function Show-Aviso([string]$Texto) {
    Add-Type -AssemblyName System.Windows.Forms
    Add-Type -AssemblyName System.Drawing

    $aviso = New-Object System.Windows.Forms.NotifyIcon
    $aviso.Icon = [System.Drawing.SystemIcons]::Information
    $aviso.BalloonTipTitle = "Gole de " + [char]0x00E1 + "gua"   # [char]0x00E1 e o "a" com acento agudo
    $aviso.BalloonTipText = $Texto
    $aviso.Visible = $true
    $aviso.ShowBalloonTip(5000)

    # O aviso some junto com o script, entao espera ele aparecer.
    Start-Sleep -Seconds 6
    $aviso.Dispose()
}

$linhas = foreach ($trilha in $Trilhas) { "${trilha}: $(Send-Gole $trilha)" }
Show-Aviso ($linhas -join "`n")
